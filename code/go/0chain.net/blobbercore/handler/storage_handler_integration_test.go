package handler

import (
	"context"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/zboxcore/client"
	"google.golang.org/grpc/metadata"
	"net/http"
	"testing"
)

func TestBlobberGRPCService_MarketplaceShareInfo(t *testing.T) {
	bClient, tdController := setupHandlerIntegrationTests(t)
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	err := tdController.ClearDatabase()
	if err != nil {
		t.Fatal(err)
	}

	err = tdController.AddMarketplaceShareInfoTestData(allocationTx, pubKey)
	if err != nil {
		t.Fatal(err)
	}

	authTicket, err := GetAuthTicketForEncryptedFile(allocationTx, "/", "exampleId:examplePath", client.GetClientID(), pubKey)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		context        metadata.MD
		input          *blobbergrpc.MarketplaceShareInfoRequest
		expectingError bool
	}{
		{
			name: "Success Insert share ",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.MarketplaceShareInfoRequest{
				Allocation:          allocationTx,
				EncryptionPublicKey: pubKey,
				AuthTicket:          authTicket,
				HttpMethod:          http.MethodPost,
				Path:                "/file.txt",
				RefereeClientId:     "",
			},
			expectingError: false,
		},
		{
			name: "Success Revoke share ",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.MarketplaceShareInfoRequest{
				Allocation:          allocationTx,
				EncryptionPublicKey: pubKey,
				AuthTicket:          authTicket,
				HttpMethod:          http.MethodDelete,
				Path:                "/file.txt",
				RefereeClientId:     "abcdefgh",
			},
			expectingError: false,
		},
		{
			name: "Invalid Marketplace share method",
			context: metadata.New(map[string]string{
				common.ClientHeader:          "exampleOwnerId",
				common.ClientSignatureHeader: clientSignature,
				common.ClientKeyHeader:       pubKey,
			}),
			input: &blobbergrpc.MarketplaceShareInfoRequest{
				Allocation:          allocationTx,
				EncryptionPublicKey: pubKey,
				AuthTicket:          authTicket,
				HttpMethod:          http.MethodGet,
				Path:                "/file.txt",
				RefereeClientId:     "abcdefgh",
			},
			expectingError: true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		ctx = metadata.NewOutgoingContext(ctx, tc.context)
		_, err := bClient.MarketplaceShareInfo(ctx, tc.input)
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
