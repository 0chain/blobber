package handler

import (
	"context"
	"encoding/hex"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_Collaborator(t *testing.T) {
	bClient, tdController := setupHandlerTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
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

	if err := tdController.ClearDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := tdController.AddCommitTestData(allocationTx, pubKey, clientId, wmSig, now); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		context         metadata.MD
		input           *blobbergrpc.CollaboratorRequest
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
			input: &blobbergrpc.CollaboratorRequest{
				Path:       "/some_file",
				PathHash:   "exampleId:examplePath",
				Allocation: allocationTx,
				Method:     http.MethodPost,
				CollabId:   "10",
			},
			expectedMessage: "Added collaborator successfully",
			expectingError:  false,
		},
		{
			name: "Fail",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.CollaboratorRequest{
				Path:       "/some_file",
				PathHash:   "exampleId:examplePath",
				Allocation: allocationTx,
				Method:     http.MethodPost,
			},
			expectedMessage: "",
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		response, err := bClient.Collaborator(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if response.GetMessage() != tc.expectedMessage {
			t.Fatal("failed!")
		}
	}
}
