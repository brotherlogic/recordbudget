package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

func init() {
	resolver.Register(&utils.DiscoveryClientResolverBuilder{})
}

func getRecord(i int32) (int32, string) {
	conn, err := grpc.Dial("discovery:///recordcollection", grpc.WithInsecure(), grpc.WithBalancerName("my_pick_first"))
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)
	ctx, cancel := utils.BuildContext("recordbudget-cli", "recordbudget")
	defer cancel()

	r, err := client.GetRecord(ctx, &rcpb.GetRecordRequest{InstanceId: i})

	if err != nil {
		return 0, fmt.Sprintf("%v", err)
	}

	return r.GetRecord().GetMetadata().GetCost(), r.GetRecord().GetRelease().GetArtists()[0].GetName() + " - " + r.GetRecord().GetRelease().GetTitle() + " [" + fmt.Sprintf("%v]", r.GetRecord().GetMetadata().GetCost())
}

func main() {
	ctx, cancel := utils.BuildContext("recordbudget-cli", "recordbudget")
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordbudget")
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewRecordBudgetServiceClient(conn)

	switch os.Args[1] {
	case "add":
		addFlags := flag.NewFlagSet("add", flag.ExitOnError)
		var name = addFlags.String("name", "", "The name of the budget")
		var btype = addFlags.String("type", "quarter", "The type of the budget")
		if err := addFlags.Parse(os.Args[2:]); err == nil {
			req := &pb.AddBudgetRequest{Name: *name}
			switch *btype {
			case "quarter":
				req.Type = pb.BudgetType_QUARTERLY
			case "infinite":
				req.Type = pb.BudgetType_INFINITE
			default:
				log.Fatalf("%v is not a know budget type", *btype)
			}

			_, err := client.AddBudget(ctx, req)
			if err != nil {
				log.Fatalf("Unable to add budget: %v", err)
			}
		}
	case "get":
		getFlags := flag.NewFlagSet("get", flag.ExitOnError)
		var name = getFlags.String("name", "", "The name of the budget")
		if err := getFlags.Parse(os.Args[2:]); err == nil {
			res, err := client.GetBudget(ctx, &pb.GetBudgetRequest{Budget: *name})
			if err != nil {
				log.Fatalf("Cannot get budget: %v", err)
			}

			fmt.Printf("Budget: %v [%v]\n", res.GetChosenBudget().GetName(), res.GetChosenBudget().GetType())
			fmt.Printf("Remaining: %v\n", res.GetChosenBudget().GetRemaining())
			sum := int32(0)
			for _, spend := range res.GetChosenBudget().GetSpends() {
				sum += spend
			}
			fmt.Sprintf("Spent: %v\n", sum)
		}
	case "sold":
		soldFlags := flag.NewFlagSet("sold", flag.ExitOnError)
		var id = soldFlags.Int("id", -1, "Id of the record to add")
		if err := soldFlags.Parse(os.Args[2:]); err == nil {
			res, err := client.GetSold(ctx, &pb.GetSoldRequest{InstanceId: int32(*id)})
			fmt.Printf("%v and %v\n", res, err)
		}
	case "budget":
		res, err := client.GetBudget(ctx, &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
		if err != nil {
			log.Fatalf("Error getting budget: %v", err)
		}
		fmt.Printf("Spend: $%v\n", res.GetSpends()/100.0)
		fmt.Printf("PreSpend: $%v\n", res.GetPreSpends()/100.0)
		fmt.Printf("Sold: $%v\n", res.GetSolds()/100.0)
		fmt.Printf("Budget: $%v\n", res.GetBudget()/100.0)
		fmt.Println("-------------")
		fmt.Printf("Budget: $%v\n", (res.GetBudget()+res.GetSolds()-res.GetSpends()-res.GetPreSpends())/100.0)
		fmt.Printf("Purchases: %v\n", len(res.GetPurchasedIds()))
	case "refresh":
		res, err := client.GetBudget(ctx, &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
		if err != nil {
			log.Fatalf("Error getting budget: %v", err)
		}
		c2 := rcpb.NewClientUpdateServiceClient(conn)
		for _, id := range res.GetPurchasedIds() {
			_, err := c2.ClientUpdate(ctx, &rcpb.ClientUpdateRequest{InstanceId: id})
			fmt.Printf("Ran %v -> %v\n", id, err)
			if err != nil {
				log.Fatalf("Can't process this one")
			}
		}
	case "ping":
		soldFlags := flag.NewFlagSet("sold", flag.ExitOnError)
		var id = soldFlags.Int("id", -1, "Id of the record to add")
		if err := soldFlags.Parse(os.Args[2:]); err == nil {
			c2 := rcpb.NewClientUpdateServiceClient(conn)
			_, err := c2.ClientUpdate(ctx, &rcpb.ClientUpdateRequest{InstanceId: int32(*id)})
			if err != nil {
				log.Fatalf("Error getting budget: %v", err)
			}
		}
	default:
		log.Fatalf("%v is not a valid option", os.Args[1])
	}
}
