package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	pbg "github.com/brotherlogic/goserver/proto"
	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

func init() {
	resolver.Register(&utils.DiscoveryServerResolverBuilder{})
}

const (
	// CONFIG storage key
	CONFIG = "/github.com/brotherlogic/recordbudget/config"
)

type rc interface {
	getRecordsSince(ctx context.Context, timeFrom int64) ([]int32, error)
	getRecord(ctx context.Context, id int32) (*rcpb.Record, error)
}

type prc struct{}

func (p *prc) getRecordsSince(ctx context.Context, since int64) ([]int32, error) {
	conn, err := grpc.Dial("discovery:///recordcollection", grpc.WithInsecure())

	if err != nil {
		return []int32{}, err
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)
	resp, err := client.QueryRecords(ctx, &rcpb.QueryRecordsRequest{Query: &rcpb.QueryRecordsRequest_UpdateTime{since}})

	if err != nil {
		return []int32{}, err
	}

	return resp.GetInstanceIds(), err
}
func (p *prc) getRecord(ctx context.Context, instanceID int32) (*rcpb.Record, error) {
	conn, err := grpc.Dial("discovery:///recordcollection", grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)
	resp, err := client.GetRecord(ctx, &rcpb.GetRecordRequest{InstanceId: instanceID})

	if err != nil {
		return nil, err
	}

	return resp.GetRecord(), err
}

//Server main server type
type Server struct {
	*goserver.GoServer
	config *pb.Config
	rc     rc
}

// Init builds the server
func Init() *Server {
	s := &Server{
		GoServer: &goserver.GoServer{},
		config:   &pb.Config{},
		rc:       &prc{},
	}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {

}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

//Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

func (s *Server) load(ctx context.Context) error {
	config := &pb.Config{}
	data, _, err := s.KSclient.Read(ctx, CONFIG, config)

	if err != nil {
		return err
	}

	config, ok := data.(*pb.Config)
	if !ok {
		return fmt.Errorf("Unable to parse config")
	}
	s.config = config
	return nil
}

func (s *Server) save(ctx context.Context) error {
	return s.KSclient.Save(ctx, CONFIG, s.config)
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.PrepServer()
	server.Register = server

	err := server.RegisterServerV2("recordbudget", false, true)
	if err != nil {
		return
	}

	server.RegisterLockingTask(server.rebuildBudget, "rebuild_budget")

	fmt.Printf("%v", server.Serve())
}
