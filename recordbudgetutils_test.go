package main

import (
	"testing"
)

func InitTestServer() *Server {
	s := Init()
	s.SkipLog = true
	return s
}

func TestSpecRead(t *testing.T) {
	s := InitTestServer()
	s.doNothing()
}
