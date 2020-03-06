package main

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
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
	s.save(context.Background())

	b, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	if err != nil {
		t.Errorf("Bad call: %v", err)
	}

	if b.GetSpends() != 100 {
		t.Errorf("Bad budget: %v", b)
	}

	if b.GetPreSpends() != 200 {
		t.Errorf("Bad budget on pre spends: %v", b)
	}
}

func TestSpendsWithFail(t *testing.T) {
	s := InitTestServer()
	s.GoServer.KSclient.Fail = true

	b, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	if err == nil {
		t.Errorf("Should have failed: %v", b)
	}
}
