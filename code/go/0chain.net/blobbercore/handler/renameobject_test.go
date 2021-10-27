package handler

import (
	"context"
	"encoding/hex"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_RenameObject(t *testing.T) {
	bClient, tdController := setupHandlerTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)

	if err := tdController.ClearDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := tdController.AddRenameTestData(allocationTx, pubKey, clientId); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		context         metadata.MD
		input           *blobbergrpc.RenameObjectRequest
		expectedMessage string
		expectingError  bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.RenameObjectRequest{
				Allocation:   allocationTx,
				Path:         "/some_file",
				PathHash:     "exampleId:examplePath",
				ConnectionId: "connection_id",
				NewName:      "some_new_file",
			},
			expectedMessage: "some_new_file",
			expectingError:  false,
		},
		{
			name: "Fail",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.RenameObjectRequest{
				Allocation:   "",
				Path:         "",
				PathHash:     "",
				ConnectionId: "",
				NewName:      "",
			},
			expectedMessage: "",
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		response, err := bClient.RenameObject(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if response.GetFilename() != tc.expectedMessage {
			t.Fatal("failed!")
		}
	}
}
