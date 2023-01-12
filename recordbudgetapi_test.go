package main

import (
	"log"
	"testing"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
	pbrc "github.com/brotherlogic/recordcollection/proto"
)

func TestSpendsWithFail(t *testing.T) {
	s := InitTestServer()
	s.GoServer.KSclient.Fail = true

	b, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	if err == nil {
		t.Errorf("Should have failed: %v", b)
	}
}

func TestBudgetAccountsSucceed(t *testing.T) {
	s := InitTestServer()

	s.AddBudget(context.Background(), &pb.AddBudgetRequest{
		Name:  "test",
		Start: time.Now().Add(-time.Hour).Unix(),
		End:   time.Now().Add(time.Hour).Unix(),
	})

	_, err := s.ClientUpdate(context.Background(), &pbrc.ClientUpdateRequest{InstanceId: 123})
	if err != nil {
		t.Errorf("Budget has not been accounted for: %v", err)
	}
}

func TestBudgetAccountsFailedBecauseOfDate(t *testing.T) {
	s := InitTestServer()

	// This should not match because of the dates
	_, err := s.AddBudget(context.Background(), &pb.AddBudgetRequest{
		Name:  "test",
		Start: time.Now().Add(2 * -time.Hour).Unix(),
		End:   time.Now().Add(-time.Hour).Unix(),
	})

	// Sniff out if the budget has been added
	if err != nil {
		t.Fatalf("Bad add of budget: %v", err)
	}
	budgets, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Budget: "test"})
	if err != nil {
		t.Fatalf("Could not retreive budget: %v, %v", budgets, err)
	}

	_, err = s.ClientUpdate(context.Background(), &pbrc.ClientUpdateRequest{InstanceId: 124})
	if err == nil {
		t.Errorf("This should have failed because of the date")
	}
	log.Printf("ERROR: %v", err)
}
