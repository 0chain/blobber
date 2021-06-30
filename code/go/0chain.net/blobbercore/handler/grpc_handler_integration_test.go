package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"google.golang.org/grpc/metadata"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const BlobberTestAddr = "localhost:7031"
const RetryAttempts = 8
const RetryTimeout = 3

func TestBlobberGRPCService_IntegrationTest(t *testing.T) {
	args := make(map[string]bool)
	for _, arg := range os.Args {
		args[arg] = true
	}
	if !args["integration"] {
		t.Skip()
	}

	var conn *grpc.ClientConn
	var err error
	for i := 0; i < RetryAttempts; i++ {
		log.Println("Connection attempt - " + fmt.Sprint(i+1))
		conn, err = grpc.Dial(BlobberTestAddr, grpc.WithInsecure())
		if err != nil {
			log.Println(err)
			<-time.After(time.Second * RetryTimeout)
			continue
		}
		break
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	blobberClient := blobbergrpc.NewBlobberClient(conn)

	setupIntegrationTestConfig(t)
	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Configuration.DBHost, config.Configuration.DBPort,
		config.Configuration.DBUserName, config.Configuration.DBName,
		config.Configuration.DBPassword)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tdController := NewTestDataController(db)

	t.Run("TestGetAllocation", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetAllocationTestData()
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name           string
			input          *blobbergrpc.GetAllocationRequest
			expectedTx     string
			expectingError bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetAllocationRequest{
					Id: "exampleTransaction",
				},
				expectedTx:     "exampleTransaction",
				expectingError: false,
			},
			{
				name: "UnknownAllocation",
				input: &blobbergrpc.GetAllocationRequest{
					Id: "exampleTransaction1",
				},
				expectedTx:     "",
				expectingError: true,
			},
		}

		for _, tc := range testCases {
			getAllocationResp, err := blobberClient.GetAllocation(context.Background(), tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getAllocationResp.Allocation.Tx != tc.expectedTx {
				t.Fatal("response with wrong allocation transaction")
			}
		}
	})

	t.Run("TestGetFileMetaData", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetFileMetaDataTestData()
		if err != nil {
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
					common.ClientHeader: "exampleOwnerId",
				}),
				input: &blobbergrpc.GetFileMetaDataRequest{
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath",
					Allocation: "exampleTransaction",
				},
				expectedFileName: "filename",
				expectingError:   false,
			},
			{
				name: "Unknown file path",
				context: metadata.New(map[string]string{
					common.ClientHeader: "exampleOwnerId",
				}),
				input: &blobbergrpc.GetFileMetaDataRequest{
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath123",
					Allocation: "exampleTransaction",
				},
				expectedFileName: "",
				expectingError:   true,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			getFileMetaDataResp, err := blobberClient.GetFileMetaData(ctx, tc.input)
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
	})

	t.Run("TestGetFileStats", func(t *testing.T) {

		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetFileStatsTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name             string
			context          metadata.MD
			input            *blobbergrpc.GetFileStatsRequest
			expectedFileName string
			expectingError   bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetFileStatsRequest{
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath",
					Allocation: allocationTx,
				},
				expectedFileName: "filename",
				expectingError:   false,
			},
			{
				name: "Unknown Path",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetFileStatsRequest{
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
			getFileStatsResp, err := blobberClient.GetFileStats(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getFileStatsResp.MetaData.FileMetaData.Name != tc.expectedFileName {
				t.Fatal("unexpected file name from GetFileStats rpc")
			}
		}

	})

	t.Run("TestListEntities", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddListEntitiesTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name           string
			context        metadata.MD
			input          *blobbergrpc.ListEntitiesRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.ListEntitiesRequest{
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath",
					AuthToken:  "",
					Allocation: allocationTx,
				},
				expectedPath:   "examplePath",
				expectingError: false,
			},
			{
				name: "bad path",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.ListEntitiesRequest{
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath123",
					AuthToken:  "",
					Allocation: allocationTx,
				},
				expectedPath:   "",
				expectingError: true,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			listEntitiesResp, err := blobberClient.ListEntities(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if listEntitiesResp.MetaData.DirMetaData.Path != tc.expectedPath {
				t.Fatal("unexpected path from ListEntities rpc")
			}
		}

	})

	t.Run("TestGetObjectPath", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetObjectPathTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name           string
			context        metadata.MD
			input          *blobbergrpc.GetObjectPathRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetObjectPathRequest{
					Allocation: allocationTx,
					Path:       "examplePath",
					BlockNum:   "0",
				},
				expectedPath:   "/",
				expectingError: false,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			getObjectPathResp, err := blobberClient.GetObjectPath(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getObjectPathResp.ObjectPath.Path.DirMetaData.Path != tc.expectedPath {
				t.Fatal("unexpected root hash from GetObjectPath rpc")
			}
		}
	})

	t.Run("TestGetReferencePath", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetReferencePathTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name           string
			context        metadata.MD
			input          *blobbergrpc.GetReferencePathRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetReferencePathRequest{
					Paths:      "",
					Path:       "/",
					Allocation: allocationTx,
				},
				expectedPath:   "/",
				expectingError: false,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			getReferencePathResp, err := blobberClient.GetReferencePath(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getReferencePathResp.ReferencePath.MetaData.DirMetaData.Path != tc.expectedPath {
				t.Fatal("unexpected path from GetReferencePath rpc")
			}
		}
	})

	t.Run("TestGetObjectTree", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetObjectTreeTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name             string
			context          metadata.MD
			input            *blobbergrpc.GetObjectTreeRequest
			expectedFileName string
			expectingError   bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetObjectTreeRequest{
					Path:       "/",
					Allocation: allocationTx,
				},
				expectedFileName: "root",
				expectingError:   false,
			},
			{
				name: "bad path",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
				}),
				input: &blobbergrpc.GetObjectTreeRequest{
					Path:       "/2",
					Allocation: "",
				},
				expectedFileName: "root",
				expectingError:   true,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			getObjectTreeResp, err := blobberClient.GetObjectTree(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getObjectTreeResp.ReferencePath.MetaData.DirMetaData.Name != tc.expectedFileName {
				t.Fatal("unexpected root name from GetObject")
			}
		}

	})

	t.Run("TestCommit", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
		pubKeyBytes, _ := hex.DecodeString(pubKey)
		clientId := encryption.Hash(pubKeyBytes)
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
			ClientID:               clientId,
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
		err = tdController.AddCommitTestData(allocationTx, pubKey, clientId, wmSig, now)
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
					common.ClientHeader:          clientId,
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
					common.ClientHeader:          clientId,
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
			getCommiteResp, err := blobberClient.Commit(ctx, tc.input)
			if err != nil {
				if !tc.expectingError {
					t.Fatal(err)
				}
				continue
			}

			if tc.expectingError {
				t.Fatal("expected error")
			}

			if getCommiteResp.WriteMarker.AllocationID != tc.expectedAllocation {
				t.Fatal("unexpected root name from GetObject")
			}
		}
	})

	t.Run("TestCommitMetaTxn", func(t *testing.T) {
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

		if err := tdController.ClearDatabase(); err != nil {
			t.Fatal(err)
		}
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
			commitMetaTxnResponse, err := blobberClient.CommitMetaTxn(ctx, tc.input)
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
	})

	t.Run("TestCollaborator", func(t *testing.T) {
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
			response, err := blobberClient.Collaborator(ctx, tc.input)
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
	})

	t.Run("TestCalculateHash", func(t *testing.T) {
		allocationTx := randString(32)
		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetReferencePathTestData(allocationTx, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		testCases := []struct {
			name           string
			context        metadata.MD
			input          *blobbergrpc.CalculateHashRequest
			expectingError bool
		}{
			{
				name: "Success",
				context: metadata.New(map[string]string{
					common.ClientHeader:          "exampleOwnerId",
					common.ClientSignatureHeader: clientSignature,
					common.ClientKeyHeader:       pubKey,
				}),
				input: &blobbergrpc.CalculateHashRequest{
					Paths:      "",
					Path:       "/",
					Allocation: allocationTx,
				},
				expectingError: false,
			},
		}

		for _, tc := range testCases {
			ctx := context.Background()
			ctx = metadata.NewOutgoingContext(ctx, tc.context)
			_, err := blobberClient.CalculateHash(ctx, tc.input)
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
	})

	t.Run("TestUpload", func(t *testing.T) {
		allocationTx := randString(32)

		pubKey, _, signScheme := GeneratePubPrivateKey(t)
		clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
		pubKeyBytes, _ := hex.DecodeString(pubKey)
		clientId := encryption.Hash(pubKeyBytes)

		formFieldByt, err := json.Marshal(&allocation.UpdateFileChange{NewFileChange: allocation.NewFileChange{Filename: `grpc_handler_integration_test.go`}})
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
		file, err := os.Open(root + "/grpc_handler_integration_test.go")
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
				expectedFileName: "grpc_handler_integration_test.go",
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
			response, err := blobberClient.UploadFile(ctx, tc.input)
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
	})
}

func randString(n int) string {

	const hexLetters = "abcdef0123456789"

	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(hexLetters[rand.Intn(len(hexLetters))])
	}
	return sb.String()
}
