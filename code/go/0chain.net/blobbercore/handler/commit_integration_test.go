package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_Commit(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientID := encryption.Hash(pubKeyBytes)
	now := common.Timestamp(time.Now().UnixNano())

	blobberPubKey := "de52c0a51872d5d2ec04dbc15a6f0696cba22657b80520e1d070e72de64c9b04e19ce3223cae3c743a20184158457582ffe9c369ca9218c04bfe83a26a62d88d"
	blobberPubKeyBytes, _ := hex.DecodeString(blobberPubKey)

	fr := reference.Ref{
		AllocationID:   "exampleId",
		Type:           "f",
		Name:           "new_name",
		Path:           "/new_name",
		ContentHash:    "contentHash",
		MerkleRoot:     "merkleRoot",
		ActualFileHash: "actualFileHash",
	}

	rootRefHash := encryption.Hash(encryption.Hash(fr.GetFileHashData()))

	wm := writemarker.WriteMarker{
		AllocationRoot:         encryption.Hash(rootRefHash + ":" + strconv.FormatInt(int64(now), 10)),
		PreviousAllocationRoot: "/",
		AllocationID:           "exampleId",
		Size:                   1337,
		BlobberID:              encryption.Hash(blobberPubKeyBytes),
		Timestamp:              now,
		ClientID:               clientID,
	}

	wmSig, err := signScheme.Sign(encryption.Hash(wm.GetHashData()))
	if err != nil {
		t.Fatal(err)
	}

	wm.Signature = wmSig

	wmRaw, err := json.Marshal(wm)
	if err != nil {
		t.Fatal(err)
	}

	err = tdController.ClearDatabase()
	if err != nil {
		t.Fatal(err)
	}
	err = tdController.AddCommitTestData(allocationTx, pubKey, clientID, wmSig, now)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name               string
		context            metadata.MD
		input              *blobbergrpc.CommitRequest
		expectedAllocation string
		expectingError     bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientID,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.CommitRequest{
				Allocation:   allocationTx,
				ConnectionId: "connection_id",
				WriteMarker:  string(wmRaw),
			},
			expectedAllocation: "exampleId",
			expectingError:     false,
		},
		{
			name: "invalid write_marker",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientID,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.CommitRequest{
				Allocation:   allocationTx,
				ConnectionId: "invalid",
				WriteMarker:  "invalid",
			},
			expectedAllocation: "",
			expectingError:     true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		getCommiteResp, err := bClient.Commit(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}
			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if getCommiteResp.WriteMarker.AllocationId != tc.expectedAllocation {
			t.Fatal("unexpected root name from GetObject")
		}
	}
}
