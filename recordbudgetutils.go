package main

import (
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordbudget/proto"
)

func (s *Server) processRec(ctx context.Context, iid int32) error {
	for _, r := range s.config.GetPurchases() {
		if r.GetInstanceId() == iid {
			return nil
		}
	}

	r, err := s.rc.getRecord(ctx, iid)
	if err != nil {
		return err
	}

	for i, pp := range s.config.GetPrePurchases() {
		if pp.GetId() == r.GetRelease().GetId() {
			s.config.PrePurchases = append(s.config.PrePurchases[:i], s.config.PrePurchases[i+1:]...)
			break
		}
	}

	s.config.Purchases = append(s.config.Purchases, &pb.BoughtRecord{InstanceId: iid, Cost: r.GetMetadata().GetCost(), BoughtDate: r.GetMetadata().GetDateAdded()})

	return nil
}

func (s *Server) rebuildBudget(ctx context.Context) (time.Time, error) {
	recs, err := s.rc.getRecordsSince(ctx, s.config.LastRecordcollectionPull)
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}

	for _, rec := range recs {
		err := s.processRec(ctx, rec)
		if err != nil {
			return time.Now().Add(time.Minute * 5), err
		}
	}

	return time.Now().Add(time.Hour * 24), err
}

func (s Server) getTotalSpend(year int) int32 {
	spend := int32(0)
	for _, purchase := range s.config.GetPurchases() {
		if time.Unix(purchase.GetBoughtDate(), 0).Year() == year {
			spend += purchase.GetCost()
		}
	}
	return spend
}
