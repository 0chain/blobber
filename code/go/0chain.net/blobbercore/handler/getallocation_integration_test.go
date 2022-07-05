package handler

import (
	"context"
	"testing"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
)

func TestGetAllocation_IntegrationTest(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)

	if err := tdController.AddGetAllocationTestData(); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		input          *blobbergrpc.GetAllocationRequest
		expectedTx     string
		expectingError bool
	}{
		{
			name: "Success",
			input: &blobbergrpc.GetAllocationRequest{
				Id: "exampleTransaction",
			},
			expectedTx:     "exampleTransaction",
			expectingError: false,
		},
		{
			name: "UnknownAllocation",
			input: &blobbergrpc.GetAllocationRequest{
				Id: "exampleTransaction1",
			},
			expectedTx:     "",
			expectingError: true,
		},
	}

	for _, tc := range testCases {
		getAllocationResp, err := bClient.GetAllocation(context.Background(), tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if getAllocationResp.Allocation.Tx != tc.expectedTx {
			t.Fatal("response with wrong allocation transaction")
		}
	}
}
