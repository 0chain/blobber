package handler

import (
	"context"
	"testing"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_GetFileMetaData(t *testing.T) {
	bClient, tdController := setupGrpcTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	if err := tdController.AddGetFileMetaDataTestData(allocationTx, pubKey); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name             string
		context          metadata.MD
		input            *blobbergrpc.GetFileMetaDataRequest
		expectedFileName string
		expectingError   bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.GetFileMetaDataRequest{
				Path:       "examplePath",
				PathHash:   "exampleId:examplePath",
				Allocation: allocationTx,
			},
			expectedFileName: "filename",
			expectingError:   false,
		},
		{
			name: "Unknown file path",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
			}),
			input: &blobbergrpc.GetFileMetaDataRequest{
				Path:       "examplePath",
				PathHash:   "exampleId:examplePath123",
				Allocation: allocationTx,
			},
			expectedFileName: "",
			expectingError:   true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		getFileMetaDataResp, err := bClient.GetFileMetaData(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if getFileMetaDataResp.MetaData.FileMetaData.Name != tc.expectedFileName {
			t.Fatal("unexpected file name from GetFileMetaData rpc")
		}
	}
}
