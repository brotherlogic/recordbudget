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
	case "budget":
		res, err := client.GetBudget(ctx, &pb.GetBudgetRequest{Year: int32(time.Now().Year())})
		if err != nil {
			log.Fatalf("Error getting budget: %v", err)
		}
		fmt.Printf("You have %v remaining in your budget, You've spent %v (%v) so far this year and have %v (%v) to come\n", res.GetBudget()-res.GetSpends(), res.GetSpends(), res.GetBudget(), res.GetPreSpends(), res.GetBudget()-res.GetSpends()-res.GetPreSpends())
		for _, p := range res.GetPurchasedIds() {
			cost, r := getRecord(p)
			if cost > 1 {
				fmt.Printf("Purchase: [%v] - %v\n", p, r)
			}
		}

		for _, p := range res.GetPrePurchasedIds() {
			fmt.Printf("PrePurchase: %v\n", p)
		}
	}
}
