package main

import (
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

const (
	// MONTHLYBUDGET Spend per month
	MONTHLYBUDGET = 400
)

func (s *Server) computeSpends(ctx context.Context, config *pb.Config, year int) (int32, int32, []int32, []int32, int32, int32) {
	resp := []int32{}
	pre := []int32{}
	spends := int32(0)
	preSpends := int32(0)
	solds := int32(0)
	for _, bought := range config.GetPurchases() {
		date := time.Unix(bought.GetBoughtDate(), 0)
		if date.Year() == year {
			resp = append(resp, bought.GetInstanceId())
			spends += bought.GetCost()
		}
	}

	for _, prebought := range config.GetPrePurchases() {
		pre = append(pre, prebought.GetId())
		preSpends += prebought.GetCost()
	}

	for _, sold := range config.GetSolds() {
		date := time.Unix(sold.GetSoldDate(), 0)
		if date.Year() == year {
			solds += sold.GetPrice()
		}
	}

	dtg := (preSpends / ((MONTHLYBUDGET * 12) / 365)) / 100

	return spends, preSpends, resp, pre, solds, int32(dtg)
}

func (s *Server) getBudget(ctx context.Context, t time.Time) int32 {
	dailyBudget := MONTHLYBUDGET * 12 / 365
	days := t.YearDay()

	return int32(days * dailyBudget * 100)
}

//GetBudget API Call
func (s *Server) GetBudget(ctx context.Context, req *pb.GetBudgetRequest) (*pb.GetBudgetResponse, error) {
	config, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.rebuildPreBudget(ctx, config)
	if err != nil {
		return nil, err
	}

	spend, preSpends, ids, pre, slds, dtg := s.computeSpends(ctx, config, int(req.GetYear()))
	budget := s.getBudget(ctx, time.Now())

	spends.Set(float64(spend))
	prespends.Set(float64(preSpends))
	alloted.Set(float64(budget))
	daysToGo.Set(float64(dtg))
	solds.Set(float64(slds))

	return &pb.GetBudgetResponse{Spends: spend, PreSpends: preSpends, Budget: budget, PurchasedIds: ids, PrePurchasedIds: pre, Solds: slds}, s.save(ctx, config)
}

//ClientUpdate on an updated record
func (s *Server) ClientUpdate(ctx context.Context, req *rcpb.ClientUpdateRequest) (*rcpb.ClientUpdateResponse, error) {
	return &rcpb.ClientUpdateResponse{}, s.processRec(ctx, req.GetInstanceId())
}
