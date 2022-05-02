package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc/resolver"

	pb "github.com/brotherlogic/recordbudget/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

func init() {
	resolver.Register(&utils.DiscoveryClientResolverBuilder{})
}

func getRecord(i int32) (int32, string) {
	ctx, cancel := utils.BuildContext("recordbudget-cli", "recordbudget")
	defer cancel()
	conn, err := utils.LFDialServer(ctx, "recordcollection")
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()

	client := rcpb.NewRecordCollectionServiceClient(conn)

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
		var sfed = addFlags.Bool("sales", false, "If the budget is fed from sales")
		if err := addFlags.Parse(os.Args[2:]); err == nil {
			req := &pb.AddBudgetRequest{Name: *name, SaleFed: *sfed}
			switch *btype {
			case "quarter":
				req.Type = pb.BudgetType_QUARTERLY
			case "infinite":
				req.Type = pb.BudgetType_INFINITE
			case "year":
				req.Type = pb.BudgetType_YEARLY
			default:
				log.Fatalf("%v is not a know budget type", *btype)
			}

			_, err := client.AddBudget(ctx, req)
			if err != nil {
				log.Fatalf("Unable to add budget: %v", err)
			}
		}
	case "seed":
		seedFlags := flag.NewFlagSet("seed", flag.ExitOnError)
		var name = seedFlags.String("name", "", "The name of the budget")
		var btype = seedFlags.String("type", "yearly", "value")
		var amount = seedFlags.Int("amount", -1, "Amount")
		if err := seedFlags.Parse(os.Args[2:]); err == nil {
			if *amount < 101 {
				log.Fatalf("You must seed with a valid amount: %v", *amount)
			}
			switch *btype {
			case "yearly":
				for i := 0; i < 12; i++ {
					dd := time.Date(time.Now().Year(), time.Month(i+1), 1, 0, 0, 0, 0, time.Local)
					_, err := client.SeedBudget(ctx, &pb.SeedBudgetRequest{
						Name:      *name,
						Timestamp: dd.Unix(),
						Amount:    int32(*amount),
					})
					if err != nil {
						log.Fatalf("Failed on %v for %v -> %v", dd, *name, err)
					}
				}
			case "once":
				_, err := client.SeedBudget(ctx, &pb.SeedBudgetRequest{
					Name:      *name,
					Timestamp: time.Now().Unix(),
					Amount:    int32(*amount),
				})
				if err != nil {
					log.Fatalf("Failed on %v for %v -> %v", time.Now().Unix(), *name, err)
				}
			default:
				log.Fatalf("Unknown seed type: %v", *btype)
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

			fmt.Printf("Budget: %v [%v] [%v]\n", res.GetChosenBudget().GetName(), res.GetChosenBudget().GetType(), res.GetChosenBudget().GetSaleFed())
			sum := int32(0)
			for _, spend := range res.GetChosenBudget().GetSpends() {
				sum += spend
			}
			fmt.Printf("Spent: %v\n", sum)
			fmt.Printf("Made: %v\n", res.GetChosenBudget().GetSolds())
			fmt.Printf("Remaining: %v\n", res.GetChosenBudget().GetRemaining())
			fmt.Printf("Seeds: %v\n", res.GetChosenBudget().GetSeeds())

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
