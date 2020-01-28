package main

import (
	"fmt"
	"testing"

	"golang.org/x/net/context"

	rcpb "github.com/brotherlogic/recordcollection/proto"
)

type trc struct {
	fail bool
}

func (t *trc) getRecordsSince(ctx context.Context, since int64) ([]int32, error) {
	if t.fail {
		return []int32{}, fmt.Errorf("Build to fail")
	}
	return []int32{12}, nil
}
func (t *trc) getRecord(ctx context.Context, instanceID int32) (*rcpb.Record, error) {
	return &rcpb.Record{}, nil
}

func InitTestServer() *Server {
	s := Init()
	s.rc = &trc{}
	s.SkipLog = true
	return s
}

func TestSpecRead(t *testing.T) {
	s := InitTestServer()
	_, err := s.rebuildBudget(context.Background())
	if err != nil {
		t.Errorf("Error on rebuild: %v", err)
	}
}

func TestSpecReadFailPull(t *testing.T) {
	s := InitTestServer()
	s.rc = &trc{fail: true}
	_, err := s.rebuildBudget(context.Background())
	if err == nil {
		t.Errorf("Error on rebuild")
	}
}
