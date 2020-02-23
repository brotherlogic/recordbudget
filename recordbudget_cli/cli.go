package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	pb "github.com/brotherlogic/recordbudget/proto"
)

func init() {
	resolver.Register(&utils.DiscoveryClientResolverBuilder{})
}

func main() {
	conn, err := grpc.Dial("discovery:///recordbudget", grpc.WithInsecure(), grpc.WithBalancerName("my_pick_first"))
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewRecordBudgetServiceClient(conn)
	ctx, cancel := utils.BuildContext("recordbudget-cli", "recordbudget")
	defer cancel()

	switch os.Args[1] {
	case "budget":
		res, err := client.GetBudget(ctx, &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
		if err != nil {
			log.Fatalf("Error getting budget: %v", err)
		}
		fmt.Printf("You have %v remaining in your budget, You've spent %v so far this year\n", res.GetBudget()-res.GetSpends(), res.GetSpends())
		for _, p := range res.GetPurchasedIds() {
			fmt.Printf("Purchase: %v\n", p)
		}
	}
}
