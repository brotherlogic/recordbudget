package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
	pbrs "github.com/brotherlogic/recordscores/proto"
)

func (s *Server) adjustDate(r *rcpb.Record) int64 {
	dateAdded := time.Unix(r.GetMetadata().GetDateAdded(), 0)
	if r.GetMetadata().GetAccountingYear() > 0 {
		dateAdded = dateAdded.AddDate(int(r.GetMetadata().GetAccountingYear())-dateAdded.Year(), 0, 0)
		s.Log(fmt.Sprintf("Adjust %v to %v", r.GetRelease().GetTitle(), dateAdded))
	}
	return dateAdded.Unix()
}

func (s *Server) processRec(ctx context.Context, iid int32) error {
	config, err := s.load(ctx)
	if err != nil {
		return err
	}

	for _, re := range config.GetSolds() {
		if re.GetInstanceId() == iid {
			return nil
		}
	}

	r, err := s.rc.getRecord(ctx, iid)
	if err != nil {
		return err
	}

	if r.GetMetadata().GetCategory() == rcpb.ReleaseMetadata_SOLD_ARCHIVE {
		conn, err := s.FDialServer(ctx, "recordscores")
		if err != nil {
			return err
		}
		rss := pbrs.NewRecordScoreServiceClient(conn)
		scores, err := rss.GetScore(ctx, &pbrs.GetScoreRequest{InstanceId: iid})
		if err != nil {
			return err
		}

		for _, score := range scores.GetScores() {
			if score.GetCategory() == rcpb.ReleaseMetadata_SOLD_ARCHIVE {
				config.Solds = append(config.Solds,
					&pb.SoldRecord{
						InstanceId: iid,
						Price:      r.GetMetadata().GetSalePrice(),
						SoldDate:   score.GetScoreTime(),
					})
				return s.save(ctx, config)
			}
		}

	}

	for _, re := range config.GetPurchases() {
		if re.GetInstanceId() == iid {
			return nil
		}
	}

	dateAdded := s.adjustDate(r)

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

func (s *Server) rebuildPreBudget(ctx context.Context, config *pb.Config) (*pb.Config, error) {
	recs, err := s.ra.getAdds(ctx)
	if err != nil {
		return nil, err
	}

	s.Log(fmt.Sprintf("Got %v adds", len(recs)))

	for _, rec := range recs {
		found := false
		for _, pre := range config.GetPrePurchases() {
			if rec.GetId() == pre.GetId() {
				found = true
			}
		}

		if !found {
			config.PrePurchases = append(config.PrePurchases, &pb.PreBoughtRecord{Id: rec.GetId(), Cost: rec.GetCost()})
		}
	}

	return config, err
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
