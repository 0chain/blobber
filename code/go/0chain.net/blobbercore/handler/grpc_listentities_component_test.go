package handler

import (
	"context"
	"testing"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_ListEntities(t *testing.T) {
	bClient, tdController := setupGrpcTests(t)

	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	err := tdController.AddListEntitiesTestData(allocationTx, pubKey)
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
			name: "Success for directory",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.ListEntitiesRequest{
				Path:       "/exampleDir",
				PathHash:   "exampleId:exampleDir",
				AuthToken:  "",
				Allocation: allocationTx,
			},
			expectedPath:   "/exampleDir",
			expectingError: false,
		},
		{
			name: "Success for file",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.ListEntitiesRequest{
				Path:       "/exampleDir/examplePath",
				PathHash:   "exampleId:examplePath",
				AuthToken:  "",
				Allocation: allocationTx,
			},
			expectedPath:   "/exampleDir",
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
		tt := tc
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			listEntitiesResp, err := bClient.ListEntities(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				return
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if listEntitiesResp.GetMetaData().GetDirMetaData().GetPath() != tc.expectedPath {
				t.Fatal("unexpected path from ListEntities rpc")
			}
		})
	}
}
