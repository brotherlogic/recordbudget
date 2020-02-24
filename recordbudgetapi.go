package main

import (
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
)

const (
	// MONTHLYBUDGET Spend per month
	MONTHLYBUDGET = 400
)

func (s *Server) computeSpends(ctx context.Context, year int) (int32, []int32, []int32) {
	resp := []int32{}
	pre := []int32{}
	spends := int32(0)
	for _, bought := range s.config.GetPurchases() {
		date := time.Unix(bought.GetBoughtDate(), 0)
		if date.Year() == year {
			resp = append(resp, bought.GetInstanceId())
			spends += bought.GetCost()
		}
	}

	for _, prebought := range s.config.GetPrePurchases() {
		pre = append(pre, prebought.GetId())
		spends += prebought.GetCost()
	}

	return spends, resp, pre
}

func (s *Server) getBudget(ctx context.Context, t time.Time) int32 {
	dailyBudget := MONTHLYBUDGET * 12 / 365
	days := t.YearDay()

	return int32(days * dailyBudget * 100)
}

//GetBudget API Call
func (s *Server) GetBudget(ctx context.Context, req *pb.GetBudgetRequest) (*pb.GetBudgetResponse, error) {
	err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	spends, ids, pre := s.computeSpends(ctx, int(req.GetYear()))
	budget := s.getBudget(ctx, time.Now())

	return &pb.GetBudgetResponse{Spends: spends, Budget: budget, PurchasedIds: ids, PrePurchasedIds: pre}, nil
}
