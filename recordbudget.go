package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	gdpb "github.com/brotherlogic/godiscogs"
	pbg "github.com/brotherlogic/goserver/proto"
	rapb "github.com/brotherlogic/recordadder/proto"
	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

var (
	budgetGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_budget",
		Help: "The size of the print queue",
	})
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

type pra struct {
	dial func(ctx context.Context, server string) (*grpc.ClientConn, error)
}

func (p *pra) getAdds(ctx context.Context) ([]*rapb.AddRecordRequest, error) {
	conn, err := p.dial(ctx, "recordadder")

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
	getOrder(ctx context.Context, id int32) (*rcpb.GetOrderResponse, error)
	updateRecord(ctx context.Context, iid int32, order *pb.Order) error
}

type prc struct {
	dial func(ctx context.Context, server string) (*grpc.ClientConn, error)
}

func (p *prc) getRecordsSince(ctx context.Context, since int64) ([]int32, error) {
	conn, err := p.dial(ctx, "recordcollection")

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
	conn, err := p.dial(ctx, "recordcollection")
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

func (p *prc) getOrder(ctx context.Context, ID int32) (*rcpb.GetOrderResponse, error) {
	conn, err := p.dial(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)
	return client.GetOrder(ctx, &rcpb.GetOrderRequest{Id: fmt.Sprintf("150295-%v", ID)})
}

func (p *prc) updateRecord(ctx context.Context, iid int32, order *pb.Order) error {
	conn, err := p.dial(ctx, "recordcollection")
	if err != nil {
		return err
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)

	_, err = client.UpdateRecord(ctx, &rcpb.UpdateRecordRequest{Reason: "final sale update", Update: &rcpb.Record{
		Release:  &gdpb.Release{InstanceId: iid},
		Metadata: &rcpb.ReleaseMetadata{SoldDate: order.GetSaleDate(), SoldPrice: order.GetSalePrice()}}})
	return err
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
	s.rc = &prc{dial: s.FDialServer}
	s.ra = &pra{dial: s.FDialServer}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterRecordBudgetServiceServer(server, s)
	rcpb.RegisterClientUpdateServiceServer(server, s)
}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

//Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

func (s *Server) load(ctx context.Context) (*pb.Config, error) {
	config := &pb.Config{}
	data, _, err := s.KSclient.Read(ctx, CONFIG, config)

	if err != nil {
		return nil, err
	}

	config, ok := data.(*pb.Config)
	if !ok {
		return nil, fmt.Errorf("Unable to parse config")
	}

	lastOrderDate.Set(float64(config.GetLastOrderPullDate()))
	orderCount.Set(float64(len(config.Orders)))

	s.updateBudgets(config)
	s.metrics(config)

	for _, budget := range config.Budgets {
		if budget.GetName() == "digital" {
			budget.Type = pb.BudgetType_YEARLY
			budget.SaleFed = false
		}
		if budget.GetName() == "digital_keep" {
			budget.SaleFed = true
		}
		if budget.GetName() == "Boosey" {
			budget.SaleFed = false
		}
	}

	return config, nil
}

func (s *Server) save(ctx context.Context, config *pb.Config) error {
	return s.KSclient.Save(ctx, CONFIG, config)
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{}
}

var (
	spends = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_spends",
		Help: "The amount of potential salve value",
	})
	prespends = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_prespends",
		Help: "The amount of potential salve value",
	})
	alloted = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_alloted",
		Help: "The amount of potential salve value",
	})
	daysToGo = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_daystogo",
		Help: "The amount of potential salve value",
	})
	solds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_solds",
		Help: "Value of sales",
	})
	orderCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_ordercount",
		Help: "Value of sales",
	})
	lastOrderDate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_last_order_date",
		Help: "Value of sales",
	})
	lastOrderNumber = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordbudget_last_order_number",
		Help: "Value of sales",
	}, []string{"response"})
	lastListing = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordbudget_last_listing",
		Help: "Value of sales",
	})
)

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.PrepServer("recordbudget")
	server.Register = server

	err := server.RegisterServerV2(false)
	if err != nil {
		return
	}

	ctx, cancel := utils.ManualContext("rb-su", time.Minute)
	server.GetBudget(ctx, &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
	cancel()

	fmt.Printf("%v", server.Serve())
}
