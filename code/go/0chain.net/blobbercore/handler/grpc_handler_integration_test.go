package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"google.golang.org/grpc/metadata"

	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"google.golang.org/grpc"
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

}
