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

func (s *Server) computeSpends(ctx context.Context, year int) int32 {
	spends := int32(0)
	for _, bought := range s.config.GetPurchases() {
		date := time.Unix(bought.GetBoughtDate(), 0)
		if date.Year() == year {
			spends += bought.GetCost()
		}
	}

	for _, prebought := range s.config.GetPrePurchases() {
		spends += prebought.GetCost()
	}

	return spends
}

func (s *Server) getBudget(ctx context.Context, t time.Time) int32 {
	dailyBudget := MONTHLYBUDGET * 12 / 365
	days := t.YearDay()

	return int32(days * dailyBudget)
}

//GetBudget API Call
func (s *Server) GetBudget(ctx context.Context, req *pb.GetBudgetRequest) (*pb.GetBudgetResponse, error) {
	spends := s.computeSpends(ctx, int(req.GetYear()))
	budget := s.getBudget(ctx, time.Now())

	return &pb.GetBudgetResponse{Spends: spends, Budget: budget}, nil
}
