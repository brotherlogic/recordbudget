package recordbudget_client

import (
	"context"

	pbgs "github.com/brotherlogic/goserver"
	pb "github.com/brotherlogic/recordbudget/proto"
)

type RecordBudgetClient struct {
	Gs      *pbgs.GoServer
	Test    bool
	budgets map[string]*pb.Budget
}

func (c *RecordBudgetClient) AddBudget(budget *pb.Budget) {
	if c.budgets == nil {
		c.budgets = make(map[string]*pb.Budget)
	}
	c.budgets[budget.GetName()] = budget
}

func (c *RecordBudgetClient) GetBudget(ctx context.Context, req *pb.GetBudgetRequest) (*pb.GetBudgetResponse, error) {
	if c.Test {
		return &pb.GetBudgetResponse{ChosenBudget: c.budgets[req.GetBudget()]}, nil
	}

	conn, err := c.Gs.FDialServer(ctx, "recordbudget")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewRecordBudgetServiceClient(conn)
	return client.GetBudget(ctx, req)
}
