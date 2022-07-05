package handler

import (
	"context"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_CommitMetaTxn(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)
	now := common.Timestamp(time.Now().UnixNano())

	blobberPubKey := "de52c0a51872d5d2ec04dbc15a6f0696cba22657b80520e1d070e72de64c9b04e19ce3223cae3c743a20184158457582ffe9c369ca9218c04bfe83a26a62d88d"
	blobberPubKeyBytes, _ := hex.DecodeString(blobberPubKey)

	fr := reference.Ref{
		AllocationID: "exampleId",
	}

	rootRefHash := encryption.Hash(encryption.Hash(fr.GetFileHashData()))

	wm := writemarker.WriteMarker{
		AllocationRoot:         encryption.Hash(rootRefHash + ":" + strconv.FormatInt(int64(now), 10)),
		PreviousAllocationRoot: "/",
		AllocationID:           "exampleId",
		Size:                   1337,
		BlobberID:              encryption.Hash(blobberPubKeyBytes),
		Timestamp:              now,
		ClientID:               clientId,
	}

	wmSig, err := signScheme.Sign(encryption.Hash(wm.GetHashData()))
	if err != nil {
		t.Fatal(err)
	}

	wm.Signature = wmSig

	if err := tdController.AddCommitTestData(allocationTx, pubKey, clientId, wmSig, now); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		context         metadata.MD
		input           *blobbergrpc.CommitMetaTxnRequest
		expectedMessage string
		expectingError  bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader: clientId,
			}),
			input: &blobbergrpc.CommitMetaTxnRequest{
				Path:       "/some_file",
				PathHash:   "exampleId:examplePath",
				AuthToken:  "",
				Allocation: allocationTx,
				TxnId:      "8",
			},
			expectedMessage: "Added commitMetaTxn successfully",
			expectingError:  false,
		},
		{
			name: "Fail",
			context: metadata.New(map[string]string{
				common.ClientHeader: clientId,
			}),
			input: &blobbergrpc.CommitMetaTxnRequest{
				Path:       "/some_file",
				PathHash:   "exampleId:examplePath",
				AuthToken:  "",
				Allocation: allocationTx,
				TxnId:      "",
			},
			expectedMessage: "",
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		commitMetaTxnResponse, err := bClient.CommitMetaTxn(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if commitMetaTxnResponse.GetMessage() != tc.expectedMessage {
			t.Fatal("failed!")
		}
	}
}
