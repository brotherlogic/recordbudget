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

func TestSpendsWithFail(t *testing.T) {
	s := InitTestServer()
	s.GoServer.KSclient.Fail = true

	b, err := s.GetBudget(context.Background(), &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	if err == nil {
		t.Errorf("Should have failed: %v", b)
	}
}
