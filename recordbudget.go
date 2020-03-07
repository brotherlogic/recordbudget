package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	pbg "github.com/brotherlogic/goserver/proto"
	rapb "github.com/brotherlogic/recordadder/proto"
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

type ra interface {
	getAdds(ctx context.Context) ([]*rapb.AddRecordRequest, error)
}

type pra struct{}

func (p *pra) getAdds(ctx context.Context) ([]*rapb.AddRecordRequest, error) {
	conn, err := grpc.Dial("discovery:///recordadder", grpc.WithInsecure())

	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := rapb.NewAddRecordServiceClient(conn)
	resp, err := client.ListQueue(ctx, &rapb.ListQueueRequest{})

	if err != nil {
		return nil, err
	}

	return resp.GetRequests(), err

}

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
	ra     ra
}

// Init builds the server
func Init() *Server {
	s := &Server{
		GoServer: &goserver.GoServer{},
		config:   &pb.Config{},
		rc:       &prc{},
		ra:       &pra{},
	}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterRecordBudgetServiceServer(server, s)
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
	s.config.LastRecordcollectionPull = time.Now().Add(-time.Hour * 24 * 100).Unix()
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
	return []*pbg.State{
		&pbg.State{Key: "purchases", Value: int64(len(s.config.GetPurchases()))},
		&pbg.State{Key: "pre_purchases", Value: int64(len(s.config.GetPrePurchases()))},
		&pbg.State{Key: fmt.Sprintf("total_spend_%v", time.Now().Year()), Value: int64(s.getTotalSpend(time.Now().Year()))},
	}
}

func (s *Server) runBudget(ctx context.Context) (time.Time, error) {
	err := s.load(ctx)
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}
	t, err := s.rebuildBudget(ctx)
	if err == nil {
		err = s.save(ctx)
	}
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}

	t, err = s.rebuildPreBudget(ctx)
	if err == nil {
		err = s.save(ctx)
	}
	if err != nil {
		return time.Now().Add(time.Minute * 5), err
	}

	s.Log(fmt.Sprintf("Have %v records in purchase, %v in pre-purchase", len(s.config.GetPurchases()), len(s.config.GetPrePurchases())))
	return t, err
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

	server.RegisterLockingTask(server.runBudget, "rebuild_budget")

	fmt.Printf("%v", server.Serve())
}
