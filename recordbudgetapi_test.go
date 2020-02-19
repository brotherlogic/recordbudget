package main

import (
	"testing"
	"time"

	pb "github.com/brotherlogic/recordbudget/proto"
	"golang.org/x/net/context"
)

func TestBasicCall(t *testing.T) {
	s := InitTestServer()

	_, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{})
	if err != nil {
		t.Errorf("Bad call: %v", err)
	}
}

func TestSpends(t *testing.T) {
	s := InitTestServer()
	s.config.Purchases = append(s.config.Purchases, &pb.BoughtRecord{BoughtDate: time.Now().Unix(), Cost: 100})
	s.config.PrePurchases = append(s.config.PrePurchases, &pb.PreBoughtRecord{Id: 12, Cost: 200})

	b, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	if err != nil {
		t.Errorf("Bad call: %v", err)
	}

	if b.GetSpends() != 300 {
		t.Errorf("Bad budget: %v", b)
	}
}
