package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
)

const BlobberAddr = "localhost:7031"
const RetryAttempts = 8
const RetryTimeout = 3

func TestBlobberGRPCService_IntegrationTest(t *testing.T) {
	args := make(map[string]bool)
	for _, arg := range os.Args {
		args[arg] = true
	}

	if !args["integration"] {
		t.Skip()
	}

	ctx := context.Background()

	var conn *grpc.ClientConn
	var err error
	for i := 0; i < RetryAttempts; i++ {
		log.Println("Connection attempt - " + fmt.Sprint(i+1))
		conn, err = grpc.Dial(BlobberAddr, grpc.WithInsecure())
		if err != nil {
			log.Println(err)
			<-time.After(time.Second * RetryTimeout)
			continue
		}

		break
	}

	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close()
	client := blobbergrpc.NewBlobberClient(conn)

	t.Run("TestGetAllocation", func(t *testing.T) {
		getAllocationReq := &blobbergrpc.GetAllocationRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "",
				ClientKey:  "",
				Allocation: "",
			},
			Id: "",
		}

		getAllocationResp, err := client.GetAllocation(ctx, getAllocationReq)
		if err != nil {
			t.Fatal(err)
		}

		if getAllocationResp.Allocation.ID != getAllocationReq.Id {
			t.Fatal("unexpected allocation id from GetAllocation rpc")
		}
	})

}
