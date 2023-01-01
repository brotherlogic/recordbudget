package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
	pbrs "github.com/brotherlogic/recordscores/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	budgets = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordbudget_budgets",
		Help: "The amount of potential salve value",
	}, []string{"budget"})
	spent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordbudget_spent",
		Help: "Total amount spent",
	}, []string{"year"})
)

func (s *Server) metrics(c *pb.Config) {
	for _, budget := range c.GetBudgets() {
		if budget.GetType() == pb.BudgetType_INFINITE {
			budgets.With(prometheus.Labels{"budget": budget.GetName()}).Set(float64(5000))
		} else {
			budgets.With(prometheus.Labels{"budget": budget.GetName()}).Set(float64(budget.GetRemaining()))
		}
	}

	yearSpend := make(map[string]int32)
	for _, spent := range c.GetPurchases() {
		year := fmt.Sprintf("%v", time.Unix(spent.GetBoughtDate(), 0).Year())
		yearSpend[year] += spent.GetCost()
	}
	for year, spend := range yearSpend {
		spent.With(prometheus.Labels{"year": year}).Set(float64(spend) / 100.0)
	}
}

func (s *Server) adjustDate(ctx context.Context, r *rcpb.Record) int64 {
	dateAdded := time.Unix(r.GetMetadata().GetDateAdded(), 0)
	if r.GetMetadata().GetAccountingYear() > 0 {
		dateAdded = dateAdded.AddDate(int(r.GetMetadata().GetAccountingYear())-dateAdded.Year(), 0, 0)
		s.CtxLog(ctx, fmt.Sprintf("Adjusting %v to %v", r.GetRelease().GetTitle(), dateAdded))
	}
	return dateAdded.Unix()
}

func (s *Server) pullOrders(ctx context.Context, config *pb.Config) (*pb.Config, error) {
	s.CtxLog(ctx, fmt.Sprintf("Pulling orders from this time %v", config.LastOrderPull))

	// Order numbers start at zero, so adjust
	if config.LastOrderPull == 0 {
		config.LastOrderPull = 1
	}
	s.CtxLog(ctx, fmt.Sprintf("Adjusted to %v", config.LastOrderPull))

	config.LastOrderPullDate = time.Now().Unix()

	order, err := s.rc.getOrder(ctx, config.LastOrderPull)
	lastOrderNumber.With(prometheus.Labels{"response": fmt.Sprintf("%v", err)}).Set(float64(config.LastOrderPull))
	if err != nil {
		if status.Convert(err).Code() == codes.FailedPrecondition {
			if config.Tracking == 0 {
				num, err := s.ImmediateIssue(ctx, "Incomplete Order Alert", fmt.Sprintf("Order %v needs completion: https://www.discogs.com/sell/order/150295-%v", config.LastOrderPull, config.LastOrderPull), true)
				if err != nil {
					return nil, err
				}
				config.Tracking = num.GetNumber()
			}
			return config, nil
		}
		if status.Convert(err).Code() == codes.NotFound {
			//Just silently ignore this - and keep moving
			return config, nil
		}

		return nil, err
	}

	if config.GetTracking() > 0 {
		s.DeleteIssue(ctx, config.GetTracking())
		config.Tracking = 0
	}

	for id, price := range order.GetListingToPrice() {
		config.Orders = append(config.Orders, &pb.Order{
			OrderId:   fmt.Sprintf("152095-%v", config.LastOrderPull),
			SaleDate:  order.GetSaleDate(),
			ListingId: id,
			SalePrice: price,
		})
		lastListing.Set(float64(id))
	}
	config.LastOrderPull++

	return config, nil
}

func (s *Server) processRec(ctx context.Context, iid int32) error {
	config, err := s.load(ctx)
	if err != nil {
		return err
	}

	if time.Now().Sub(time.Unix(config.GetLastOrderPullDate(), 0)) > time.Hour {
		config, err := s.pullOrders(ctx, config)
		if err != nil {
			return err
		}
		s.save(ctx, config)
	}

	r, err := s.rc.getRecord(ctx, iid)
	if err != nil {
		if status.Convert(err).Code() == codes.OutOfRange {
			pc := make([]*pb.BoughtRecord, 0)
			for _, b := range config.GetPurchases() {
				if b.GetInstanceId() != iid {
					pc = append(pc, b)
				}
			}
			config.Purchases = pc
			return s.save(ctx, config)
		}

		return err
	}

	// See if we've got an confirmed order for this

	if r.GetMetadata().GetSoldDate() == 0 && r.GetMetadata().GetSaleId() > 0 {
		for _, order := range config.GetOrders() {
			if order.GetListingId() == r.GetMetadata().GetSaleId() {
				err := s.rc.updateRecord(ctx, iid, order)
				s.CtxLog(ctx, fmt.Sprintf("Trying %v and %v -> %v", r.GetMetadata().GetSoldDate(), r.GetMetadata().GetSaleId(), err))
				if err != nil {
					return err
				}
			}
		}

		if r.GetMetadata().GetCategory() == rcpb.ReleaseMetadata_SOLD_ARCHIVE {
			s.RaiseIssue(fmt.Sprintf("A Difficult Sale for %v", iid), fmt.Sprintf("%v has a sale id but no related order - see https://www.discogs.com/madeup/release/%v", iid, r.GetRelease().GetId()))
		}
	}

	for _, re := range config.GetSolds() {
		if re.GetInstanceId() == iid {
			if r.GetMetadata().GetSoldPrice() > 0 {
				re.Price = r.GetMetadata().GetSoldPrice()
				re.SoldDate = r.GetMetadata().GetSoldDate()
				return s.save(ctx, config)
			}
			return nil
		}
	}

	if r.GetMetadata().GetCategory() == rcpb.ReleaseMetadata_SOLD_ARCHIVE {
		conn, err := s.FDialServer(ctx, "recordscores")
		if err != nil {
			return err
		}
		defer conn.Close()
		rss := pbrs.NewRecordScoreServiceClient(conn)
		scores, err := rss.GetScore(ctx, &pbrs.GetScoreRequest{InstanceId: iid})
		if err != nil {
			return err
		}

		for _, score := range scores.GetScores() {
			if score.GetCategory() == rcpb.ReleaseMetadata_SOLD_ARCHIVE {
				if r.GetMetadata().GetSoldPrice() > 0 {
					config.Solds = append(config.Solds,
						&pb.SoldRecord{
							InstanceId: iid,
							Price:      r.GetMetadata().GetSoldPrice(),
							SoldDate:   r.GetMetadata().GetSoldDate(),
						})
				} else {
					config.Solds = append(config.Solds,
						&pb.SoldRecord{
							InstanceId: iid,
							Price:      r.GetMetadata().GetSalePrice(),
							SoldDate:   score.GetScoreTime(),
						})
				}
				return s.save(ctx, config)
			}
		}

	}

	for _, re := range config.GetPurchases() {
		if re.GetInstanceId() == iid {
			if re.GetBudget() != r.GetMetadata().GetPurchaseBudget() {
				re.Budget = r.GetMetadata().GetPurchaseBudget()
				return s.save(ctx, config)
			}
			return nil
		}
	}

	dateAdded := s.adjustDate(ctx, r)

	for i, pp := range config.GetPrePurchases() {
		if pp.GetId() == r.GetRelease().GetId() {
			config.PrePurchases = append(config.PrePurchases[:i], config.PrePurchases[i+1:]...)
			break
		}
	}

	config.Purchases = append(config.Purchases, &pb.BoughtRecord{InstanceId: iid, Cost: r.GetMetadata().GetCost(), BoughtDate: dateAdded})

	return s.save(ctx, config)
}

func (s *Server) rebuildBudget(ctx context.Context) (time.Time, error) {
	recs, err := s.rc.getRecordsSince(ctx, s.config.LastRecordcollectionPull)
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}

	for _, rec := range recs {
		err := s.processRec(ctx, rec)
		if err != nil {
			return time.Now().Add(time.Minute * 5), err
		}
	}

	return time.Now().Add(time.Hour * 1), err
}

func (s Server) getTotalSpend(year int) int32 {
	spend := int32(0)
	for _, purchase := range s.config.GetPurchases() {
		if time.Unix(purchase.GetBoughtDate(), 0).Year() == year {
			spend += purchase.GetCost()
		}
	}
	return spend
}

func (s *Server) adjustBudget(ctx context.Context, budget *pb.Budget, config *pb.Config) {
	for _, purchase := range config.GetPurchases() {
		if purchase.GetBudget() == budget.GetName() {
			if budget.Spends == nil {
				budget.Spends = make(map[int32]int32)
			}
			budget.Spends[purchase.GetInstanceId()] = purchase.Cost
		}
	}

	if budget.GetSaleFed() {
		sfcount := int32(0)
		for _, budget := range config.GetBudgets() {
			if budget.SaleFed {
				sfcount++
			}
		}

		solds := int32(0)
		for _, sale := range config.Solds {
			if time.Unix(sale.GetSoldDate(), 0).Year() == time.Now().Year() {
				solds += sale.GetPrice()
			}
		}
		s.CtxLog(ctx, fmt.Sprintf("Found %v in sales with %v sale fed budgets", solds, sfcount))
		budget.Solds = solds / sfcount
	}

	s.updateBudgets(config)
}
