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
	rcpb "github.com/brotherlogic/recordcollection/proto"
)

func init() {
	resolver.Register(&utils.DiscoveryClientResolverBuilder{})
}

func getRecord(i int32) string {
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
		return fmt.Sprintf("%v", err)
	}

	return r.GetRecord().GetRelease().GetArtists()[0].GetName() + " - " + r.GetRecord().GetRelease().GetTitle() + "[" + fmt.Sprintf("%v]", r.GetRecord().GetMetadata().GetCost())
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
		fmt.Printf("You have %v remaining in your budget, You've spent %v (%v) so far this year\n", res.GetBudget()-res.GetSpends(), res.GetSpends(), res.GetBudget())
		for _, p := range res.GetPurchasedIds() {
			fmt.Printf("Purchase: [%v] - %v\n", p, getRecord(p))
		}

		for _, p := range res.GetPrePurchasedIds() {
			fmt.Printf("PrePurchase: %v\n", p)
		}
	}
}
