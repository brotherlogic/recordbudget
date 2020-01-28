package main

import (
	"time"

	"golang.org/x/net/context"
)

func (s *Server) rebuildBudget(ctx context.Context) (time.Time, error) {
	_, err := s.rc.getRecordsSince(ctx, s.config.LastRecordcollectionPull)
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}

	return time.Now().Add(time.Hour * 24), err
}
