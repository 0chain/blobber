package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/DATA-DOG/go-sqlmock"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"
)

func setupMockForFileManager() error {
	mock := datastore.MockTheStore(nil)

	aa := sqlmock.AnyArg()
	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations"`)).
		WillReturnRows(sqlmock.NewRows(
			[]string{
				"id", "blobber_size", "blobber_size_used",
			},
		).AddRow(
			"allocation id", 655360000, 6553600,
		),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reference_objects" WHERE`)).
		WithArgs(aa, aa, aa).
		WillReturnRows(
			sqlmock.NewRows([]string{"count"}).AddRow(1000),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT sum(size) as file_size FROM "reference_objects" WHERE`)).
		WillReturnRows(
			sqlmock.NewRows([]string{"file_size"}).AddRow(6553600),
		)

	mock.ExpectClose()
	return nil
}

func setupFileManager(mp string) error {
	fs := &MockFileStore{mp: mp}
	err := fs.Initialize()
	if err != nil {
		return err
	}
	filestore.SetFileStore(fs)
	return nil
}

func TestBlobberGRPCService_DownloadFile(t *testing.T) {
	if err := setupMockForFileManager(); err != nil {
		t.Fatal(err)
	}

	mp := filepath.Join(os.TempDir(), "/test_dl_files")
	if err := os.MkdirAll(mp, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err := setupFileManager(mp); err != nil {
		t.Fatal(err)
	}

	allocID := randString(64)
	allocTx := randString(64)
	pathHash := randString(64)
	contentHash := randString(64)

	fPath, err := filestore.GetFileStore().GetPathForFile(allocID, contentHash)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Dir(fPath), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := os.RemoveAll(mp)
		if err != nil {
			t.Fatal(err)
		}
	}()

	f, err := os.Create(fPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	file, err := os.CreateTemp(mp, "test*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		t.Fatal(err)
	}

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocTx))
	pubKeyBytes, _ := hex.DecodeString(pubKey)
	clientId := encryption.Hash(pubKeyBytes)
	now := common.Timestamp(time.Now().Unix())

	bClient, tdController := setupGrpcTests(t)

	blobberPubKey := "de52c0a51872d5d2ec04dbc15a6f0696cba22657b80520e1d070e72de64c9b04e19ce3223cae3c743a20184158457582ffe9c369ca9218c04bfe83a26a62d88d"
	blobberPubKeyBytes, _ := hex.DecodeString(blobberPubKey)

	node.Self.ID = encryption.Hash(blobberPubKeyBytes)
	node.Self.PublicKey = blobberPubKey

	rm := readmarker.ReadMarker{
		BlobberID:       encryption.Hash(blobberPubKeyBytes),
		AllocationID:    allocID,
		ClientPublicKey: pubKey,
		ClientID:        clientId,
		OwnerID:         clientId,
		Timestamp:       now,
		ReadCounter:     1,
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

	if err := tdController.AddDownloadTestData(allocID, allocTx, pathHash, contentHash, pubKey, clientId, rmSig, now); err != nil {
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
				Allocation: allocTx,
				Path:       "/some_file",
				PathHash:   pathHash,
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
