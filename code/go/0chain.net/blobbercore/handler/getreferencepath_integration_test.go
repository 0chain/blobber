package handler

import (
	"context"
	"os"
	"testing"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_GetReferencePath(t *testing.T) {
	if os.Getenv("integration") != "1" {
		t.Skip()
	}

	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	err := tdController.ClearDatabase()
	if err != nil {
		t.Fatal(err)
	}
	err = tdController.AddGetReferencePathTestData(allocationTx, pubKey)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		context        metadata.MD
		input          *blobbergrpc.GetReferencePathRequest
		expectedPath   string
		expectingError bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.GetReferencePathRequest{
				Paths:      "",
				Path:       "/",
				Allocation: allocationTx,
			},
			expectedPath:   "/",
			expectingError: false,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		getReferencePathResp, err := bClient.GetReferencePath(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if getReferencePathResp.ReferencePath.MetaData.DirMetaData.Path != tc.expectedPath {
			t.Fatal("unexpected path from GetReferencePath rpc")
		}
	}
}
