package handler

import (
	"context"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"testing"


	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_ListEntities(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)

	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	err := tdController.ClearDatabase()
	if err != nil {
		t.Fatal(err)
	}
	err = tdController.AddListEntitiesTestData(allocationTx, pubKey)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		context        metadata.MD
		input          *blobbergrpc.ListEntitiesRequest
		expectedPath   string
		expectingError bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.ListEntitiesRequest{
				Path:       "examplePath",
				PathHash:   "exampleId:examplePath",
				AuthToken:  "",
				Allocation: allocationTx,
			},
			expectedPath:   "examplePath",
			expectingError: false,
		},
		{
			name: "bad path",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.ListEntitiesRequest{
				Path:       "examplePath",
				PathHash:   "exampleId:examplePath123",
				AuthToken:  "",
				Allocation: allocationTx,
			},
			expectedPath:   "",
			expectingError: true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		listEntitiesResp, err := bClient.ListEntities(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if listEntitiesResp.MetaData.DirMetaData.Path != tc.expectedPath {
			t.Fatal("unexpected path from ListEntities rpc")
		}
	}

}
