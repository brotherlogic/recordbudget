package main

import (
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	dtg := ((preSpends) / ((MONTHLYBUDGET * 12) / 365)) / 100

	return spends, preSpends, resp, pre, solds, int32(dtg)
}

func (s *Server) getBudget(ctx context.Context, t time.Time) int32 {
	dailyBudget := MONTHLYBUDGET * 12 / 365
	days := t.YearDay()

	return int32(days * dailyBudget * 100)
}

func (s *Server) updateBudgets(config *pb.Config) {
	for _, budget := range config.GetBudgets() {
		spent := int32(0)
		made := int32(0)
		for d, m := range budget.GetSeeds() {
			if time.Since(time.Unix(d, 0)) > time.Second {
				made += m
			}
		}

		for _, sp := range budget.GetSpends() {
			spent += sp
		}
		budget.Remaining = made - spent + budget.GetSolds()
	}
}

// GetBudget API Call
func (s *Server) GetBudget(ctx context.Context, req *pb.GetBudgetRequest) (*pb.GetBudgetResponse, error) {
	config, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	for _, budget := range config.GetBudgets() {
		if budget.GetName() == req.GetBudget() {
			s.adjustBudget(ctx, budget, config)
			return &pb.GetBudgetResponse{ChosenBudget: budget}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "The budget %v was not found", req.GetBudget())
}

// ClientUpdate on an updated record
func (s *Server) ClientUpdate(ctx context.Context, req *rcpb.ClientUpdateRequest) (*rcpb.ClientUpdateResponse, error) {
	return &rcpb.ClientUpdateResponse{}, s.processRec(ctx, req.GetInstanceId())
}

func (s *Server) GetSold(ctx context.Context, req *pb.GetSoldRequest) (*pb.GetSoldResponse, error) {
	config, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	var solds []*pb.SoldRecord
	for _, sold := range config.GetSolds() {
		if req.GetInstanceId() == 0 && sold.GetInstanceId() == req.GetInstanceId() {
			solds = append(solds, sold)
		}
	}

	return &pb.GetSoldResponse{Record: solds}, nil
}

func (s *Server) AddBudget(ctx context.Context, req *pb.AddBudgetRequest) (*pb.AddBudgetResponse, error) {
	config, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	for _, budget := range config.GetBudgets() {
		if budget.GetName() == req.GetName() {
			if budget.GetType() != req.GetType() {
				budget.Type = req.GetType()
				return &pb.AddBudgetResponse{}, s.save(ctx, config)
			}
			return nil, status.Errorf(codes.AlreadyExists, "%v already exists", req.GetName())
		}
	}

	config.Budgets = append(config.Budgets, &pb.Budget{
		Name:  req.GetName(),
		Type:  req.GetType(),
		Start: req.GetStart(),
		End:   getEnd(req.GetEnd(), req.GetType()),
	})

	return &pb.AddBudgetResponse{}, s.save(ctx, config)
}

func getEnd(val int64, ty pb.BudgetType) int64 {
	if val > 0 {
		return val
	}
	if ty == pb.BudgetType_YEARLY {
		return time.Date(time.Now().Year(), time.December, 31, 23, 59, 59, 0, time.Now().Location()).Unix()
	}
	return 0
}

func (s Server) SeedBudget(ctx context.Context, req *pb.SeedBudgetRequest) (*pb.SeedBudgetResponse, error) {
	config, err := s.load(ctx)
	if err != nil {
		return nil, err
	}

	for _, budget := range config.GetBudgets() {
		if budget.GetName() == req.GetName() {
			if budget.Seeds == nil {
				budget.Seeds = make(map[int64]int32)
			}
			budget.Seeds[req.GetTimestamp()] = req.GetAmount()
			return &pb.SeedBudgetResponse{}, s.save(ctx, config)
		}
	}

	return nil, status.Errorf(codes.FailedPrecondition, "%v is not a valid budget", req.GetName())
}
