package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_UpdateObjectAttributes(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)

	if err := tdController.AddAttributesTestData(allocationTx, pubKey, clientId); err != nil {
		t.Fatal(err)
	}

	attr := &reference.Attributes{WhoPaysForReads: common.WhoPays3rdParty}
	attrBytes, err := json.Marshal(attr)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		context         metadata.MD
		input           *blobbergrpc.UpdateObjectAttributesRequest
		expectedMessage int
		expectingError  bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.UpdateObjectAttributesRequest{
				Allocation:   allocationTx,
				Path:         "/some_file",
				PathHash:     "exampleId:examplePath",
				ConnectionId: "connection_id",
				Attributes:   string(attrBytes),
			},
			expectedMessage: int(attr.WhoPaysForReads),
			expectingError:  false,
		},
		{
			name: "Fail",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.UpdateObjectAttributesRequest{
				Allocation:   "",
				Path:         "",
				PathHash:     "",
				ConnectionId: "",
				Attributes:   "",
			},
			expectedMessage: 0,
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		response, err := bClient.UpdateObjectAttributes(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if response.GetWhoPaysForReads() != int64(tc.expectedMessage) {
			t.Fatal("failed!")
		}
	}
}
