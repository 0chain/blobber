package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"io"
	"os"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_UploadFile(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)

	formFieldByt, err := json.Marshal(&allocation.UpdateFileChange{NewFileChange: allocation.NewFileChange{Filename: `helper_integration_test.go`}})
	if err != nil {
		t.Fatal(err)
	}

	if err := tdController.ClearDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := tdController.AddUploadTestData(allocationTx, pubKey, clientId); err != nil {
		t.Fatal(err)
	}

	root, _ := os.Getwd()
	file, err := os.Open(root + "/helper_integration_test.go")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := file.Stat()
	if err != nil {
		panic(err)
	}
	fileB := make([]byte, stats.Size())
	if _, err := io.ReadFull(file, fileB); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name             string
		context          metadata.MD
		input            *blobbergrpc.UploadFileRequest
		expectedFileName string
		expectingError   bool
	}{
		{
			name: "Success",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),

			input: &blobbergrpc.UploadFileRequest{
				Allocation:          allocationTx,
				Path:                "/some_file",
				ConnectionId:        "connection_id",
				Method:              "POST",
				UploadMeta:          string(formFieldByt),
				UpdateMeta:          "",
				UploadFile:          fileB,
				UploadThumbnailFile: []byte{},
			},
			expectedFileName: "helper_integration_test.go",
			expectingError:   false,
		},
		{
			name: "Fail",
			context: metadata.New(map[string]string{
				common.ClientHeader:          clientId,
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.UploadFileRequest{
				Allocation:          "",
				Path:                "",
				ConnectionId:        "",
				Method:              "",
				UploadMeta:          "",
				UpdateMeta:          "",
				UploadFile:          nil,
				UploadThumbnailFile: nil,
			},
			expectedFileName: "",
			expectingError:   true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		response, err := bClient.UploadFile(ctx, tc.input)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if response.GetFilename() != tc.expectedFileName {
			t.Fatal("failed!")
		}
	}
}
