package main

import (
	"fmt"
	"testing"
	"time"

	keystoreclient "github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"

	gdpb "github.com/brotherlogic/godiscogs"
	rapb "github.com/brotherlogic/recordadder/proto"
	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

type tra struct {
	fail bool
}

func (t *tra) getAdds(ctx context.Context) ([]*rapb.AddRecordRequest, error) {
	if t.fail {
		return []*rapb.AddRecordRequest{}, fmt.Errorf("Built to fail")
	}

	return []*rapb.AddRecordRequest{&rapb.AddRecordRequest{Id: 12}}, nil
}

type trc struct {
	fail    bool
	getFail bool
}

func (t *trc) getRecordsSince(ctx context.Context, since int64) ([]int32, error) {
	if t.fail {
		return []int32{}, fmt.Errorf("Build to fail")
	}
	return []int32{12}, nil
}
func (t *trc) getRecord(ctx context.Context, instanceID int32) (*rcpb.Record, error) {
	if t.getFail {
		return nil, fmt.Errorf("Built to fail")
	}
	return &rcpb.Record{Release: &gdpb.Release{Id: 12}}, nil
}

func (t *trc) getOrder(ctx context.Context, ID int32) (*rcpb.GetOrderResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func InitTestServer() *Server {
	s := Init()
	s.rc = &trc{}
	s.ra = &tra{}
	s.SkipLog = true
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".test")
	s.GoServer.KSclient.Save(context.Background(), CONFIG, &pb.Config{})
	return s
}

func TestSpecRead(t *testing.T) {
	s := InitTestServer()
	_, err := s.rebuildBudget(context.Background())
	if err != nil {
		t.Errorf("Error on rebuild: %v", err)
	}
}

func TestSpecReadWithBadRecordRead(t *testing.T) {
	s := InitTestServer()
	s.rc = &trc{getFail: true}
	_, err := s.rebuildBudget(context.Background())
	if err == nil {
		t.Errorf("No error with bad build")
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

func TestProcessExistingRec(t *testing.T) {
	s := InitTestServer()
	s.config.PrePurchases = append(s.config.PrePurchases, &pb.PreBoughtRecord{Id: 12})

	err := s.processRec(context.Background(), 12)
	if err != nil {
		t.Errorf("Bad process: %v", err)
	}
	err = s.processRec(context.Background(), 12)
	if err != nil {
		t.Errorf("Bad process again: %v", err)
	}

	if len(s.config.GetPurchases()) != 1 {
		t.Errorf("Bad adds: %v", s.config)
	}
}

func TestProcessRecWithGetFail(t *testing.T) {
	s := InitTestServer()
	s.rc = &trc{getFail: true}

	err := s.processRec(context.Background(), 12)

	if err == nil {
		t.Errorf("Bad proc did not fail")
	}
}

func TestGetSpend(t *testing.T) {
	s := InitTestServer()
	s.config.Purchases = append(s.config.Purchases, &pb.BoughtRecord{BoughtDate: time.Now().Unix(), Cost: 100})

	val := s.getTotalSpend(time.Now().Year())

	if val != 100 {
		t.Errorf("Bad calc: %v", val)
	}
}

func TestFailPre(t *testing.T) {
	s := InitTestServer()
	s.ra = &tra{fail: true}

	_, err := s.rebuildPreBudget(context.Background(), &pb.Config{})

	if err == nil {
		t.Errorf("Bad ra did not fail")
	}
}

func TestPre(t *testing.T) {
	s := InitTestServer()

	_, err := s.rebuildPreBudget(context.Background(), &pb.Config{})
	if err != nil {
		t.Errorf("Bad rebuild: %v", err)
	}

	_, err = s.rebuildPreBudget(context.Background(), &pb.Config{})
	if err != nil {
		t.Errorf("Bad rebuild: %v", err)
	}

	if len(s.config.GetPrePurchases()) != 1 {
		t.Errorf("Bad adds: %v", s.config)
	}
}

func TestAdjust(t *testing.T) {
	s := InitTestServer()

	dateAdded, err := time.Parse("2006-01-02", "2019-03-17")
	if err != nil {
		t.Fatalf("Error parse: %v", err)
	}

	d2 := s.adjustDate(&rcpb.Record{Metadata: &rcpb.ReleaseMetadata{DateAdded: dateAdded.Unix()}})
	if d2 != dateAdded.Unix() {
		t.Errorf("Mismatch: %v and %v", dateAdded, d2)
	}
}

func TestAdjustDoAdjust(t *testing.T) {
	s := InitTestServer()

	dateAdded, err := time.Parse("2006-01-02", "2019-03-17")
	dateAdded2, err := time.Parse("2006-01-02", "2018-03-17")
	if err != nil {
		t.Fatalf("Error parse: %v", err)
	}

	d2 := s.adjustDate(&rcpb.Record{Metadata: &rcpb.ReleaseMetadata{DateAdded: dateAdded.Unix(), AccountingYear: int32(2018)}})
	if d2 != dateAdded2.Unix() {
		t.Errorf("Mismatch: %v and %v", dateAdded, d2)
	}
}
