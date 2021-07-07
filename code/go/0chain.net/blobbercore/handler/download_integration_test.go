package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_DownloadFile(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	root, _ := os.Getwd()
	path := strings.Split(root, `code`)

	err := os.MkdirAll(path[0]+`docker.local/blobber1/files/files/exa/mpl/eId/objects/tmp/Mon/Wen`, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(path[0] + `docker.local/blobber1/files/files/exa/mpl/eId/objects/tmp/Mon`)
		if err != nil {
			t.Fatal(err)
		}
	}()

	f, err := os.Create(path[0] + `docker.local/blobber1/files/files/exa/mpl/eId/objects/tmp/Mon/Wen/MyFile`)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	file, err := os.Open(root + "/helper_integration_test.go")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		t.Fatal(err)
	}

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)
	now := common.Timestamp(time.Now().Unix())
	allocationId := `exampleId`

	if err := tdController.ClearDatabase(); err != nil {
		t.Fatal(err)
	}

	blobberPubKey := "de52c0a51872d5d2ec04dbc15a6f0696cba22657b80520e1d070e72de64c9b04e19ce3223cae3c743a20184158457582ffe9c369ca9218c04bfe83a26a62d88d"
	blobberPubKeyBytes, _ := hex.DecodeString(blobberPubKey)

	rm := readmarker.ReadMarker{
		BlobberID:       encryption.Hash(blobberPubKeyBytes),
		AllocationID:    allocationId,
		ClientPublicKey: pubKey,
		ClientID:        clientId,
		OwnerID:         clientId,
		Timestamp:       now,
		//ReadCounter:     1337,
	}

	rmSig, err := signScheme.Sign(encryption.Hash(rm.GetHashData()))
	if err != nil {
		t.Fatal(err)
	}
	rm.Signature = rmSig

	rmString, err := json.Marshal(rm)
	if err != nil {
		t.Fatal(err)
	}

	if err := tdController.AddDownloadTestData(allocationTx, pubKey, clientId, rmSig, now); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		context         metadata.MD
		input           *blobbergrpc.DownloadFileRequest
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
			input: &blobbergrpc.DownloadFileRequest{
				Allocation: allocationTx,
				Path:       "/some_file",
				PathHash:   "exampleId:examplePath",
				ReadMarker: string(rmString),
				BlockNum:   "1",
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
			input: &blobbergrpc.DownloadFileRequest{
				Allocation: "",
				Path:       "",
				PathHash:   "",
				RxPay:      "",
				BlockNum:   "",
				NumBlocks:  "",
				ReadMarker: "",
				AuthToken:  "",
				Content:    "",
			},
			expectedMessage: "",
			expectingError:  true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		_, err := bClient.DownloadFile(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}
	}
}
