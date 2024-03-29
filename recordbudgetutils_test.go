package main

import (
	"fmt"
	"testing"
	"time"

	keystoreclient "github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"

	gdpb "github.com/brotherlogic/godiscogs/proto"
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
	if instanceID == 123 {
		return &rcpb.Record{Release: &gdpb.Release{Id: 12}, Metadata: &rcpb.ReleaseMetadata{PurchaseBudget: "test", DateAdded: time.Now().Unix()}}, nil
	}
	if instanceID == 124 {
		return &rcpb.Record{Release: &gdpb.Release{Id: 14}, Metadata: &rcpb.ReleaseMetadata{PurchaseBudget: "test", DateAdded: time.Now().Unix()}}, nil
	}
	if instanceID == 125 {
		return &rcpb.Record{Release: &gdpb.Release{Id: 14}, Metadata: &rcpb.ReleaseMetadata{PurchaseBudget: "test", DateAdded: time.Now().Unix(), AccountingYear: 2020}}, nil
	}
	return &rcpb.Record{Release: &gdpb.Release{Id: 12}}, nil
}

func (t *trc) getOrder(ctx context.Context, ID int32) (*rcpb.GetOrderResponse, error) {
	return &rcpb.GetOrderResponse{}, nil
}

func (t *trc) updateRecord(ctx context.Context, iid int32, order *pb.Order) error {
	return nil
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

func TestAdjust(t *testing.T) {
	s := InitTestServer()

	dateAdded, err := time.Parse("2006-01-02", "2019-03-17")
	if err != nil {
		t.Fatalf("Error parse: %v", err)
	}

	d2 := s.adjustDate(context.Background(), &rcpb.Record{Metadata: &rcpb.ReleaseMetadata{DateAdded: dateAdded.Unix()}})
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

	d2 := s.adjustDate(context.Background(), &rcpb.Record{Metadata: &rcpb.ReleaseMetadata{DateAdded: dateAdded.Unix(), AccountingYear: int32(2018)}})
	if d2 != dateAdded2.Unix() {
		t.Errorf("Mismatch: %v and %v", dateAdded, d2)
	}
}
