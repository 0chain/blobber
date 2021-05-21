package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"github.com/spf13/viper"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"google.golang.org/grpc"
)

const BlobberAddr = "localhost:7031"
const RetryAttempts = 8
const RetryTimeout = 3

func TestBlobberGRPCService_IntegrationTest(t *testing.T) {
	args := make(map[string]bool)
	for _, arg := range os.Args {
		args[arg] = true
	}
	if !args["integration"] {
		//t.Skip()
	}

	ctx := context.Background()

	var conn *grpc.ClientConn
	var err error
	for i := 0; i < RetryAttempts; i++ {
		log.Println("Connection attempt - " + fmt.Sprint(i+1))
		conn, err = grpc.Dial(BlobberAddr, grpc.WithInsecure())
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

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	configDir := strings.Split(pwd, "/code/go")[0] + "/config"
	config.SetupDefaultConfig()
	config.SetupConfig(configDir)
	config.Configuration.DBHost = "localhost"
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
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
					Context: &blobbergrpc.RequestContext{},
					Id:      "exampleTransaction",
				},
				expectedTx:     "exampleTransaction",
				expectingError: false,
			},
			{
				name: "UnknownAllocation",
				input: &blobbergrpc.GetAllocationRequest{
					Context: &blobbergrpc.RequestContext{},
					Id:      "exampleTransaction1",
				},
				expectedTx:     "",
				expectingError: true,
			},
		}

		for _, tc := range testCases {
			getAllocationResp, err := blobberClient.GetAllocation(ctx, tc.input)
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
			input            *blobbergrpc.GetFileMetaDataRequest
			expectedFileName string
			expectingError   bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetFileMetaDataRequest{
					Context: &blobbergrpc.RequestContext{
						Client:     "exampleOwnerId",
						Allocation: "exampleTransaction",
					},
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath",
					Allocation: "exampleTransaction",
				},
				expectedFileName: "filename",
				expectingError:   false,
			},
			{
				name: "Unknown file path",
				input: &blobbergrpc.GetFileMetaDataRequest{
					Context: &blobbergrpc.RequestContext{
						Client:     "exampleOwnerId",
						Allocation: "exampleTransaction",
					},
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath123",
					Allocation: "exampleTransaction",
				},
				expectedFileName: "",
				expectingError:   true,
			},
		}

		for _, tc := range testCases {
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
			input            *blobbergrpc.GetFileStatsRequest
			expectedFileName string
			expectingError   bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetFileStatsRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:     "examplePath",
					PathHash: "exampleId:examplePath",
				},
				expectedFileName: "filename",
				expectingError:   false,
			},
			{
				name: "Unknown Path",
				input: &blobbergrpc.GetFileStatsRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:     "examplePath",
					PathHash: "exampleId:examplePath123",
				},
				expectedFileName: "",
				expectingError:   true,
			},
		}

		for _, tc := range testCases {
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
			input          *blobbergrpc.ListEntitiesRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.ListEntitiesRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath",
					AuthToken:  "",
					Allocation: "",
				},
				expectedPath:   "examplePath",
				expectingError: false,
			},
			{
				name: "bad path",
				input: &blobbergrpc.ListEntitiesRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:       "examplePath",
					PathHash:   "exampleId:examplePath123",
					AuthToken:  "",
					Allocation: "",
				},
				expectedPath:   "",
				expectingError: true,
			},
		}

		for _, tc := range testCases {
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
			input          *blobbergrpc.GetObjectPathRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetObjectPathRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Allocation: "",
					Path:       "examplePath",
					BlockNum:   "0",
				},
				expectedPath:   "/",
				expectingError: false,
			},
		}

		for _, tc := range testCases {
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
			input          *blobbergrpc.GetReferencePathRequest
			expectedPath   string
			expectingError bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetReferencePathRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Paths:      "",
					Path:       "/",
					Allocation: "",
				},
				expectedPath:   "/",
				expectingError: false,
			},
		}

		for _, tc := range testCases {
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
			input            *blobbergrpc.GetObjectTreeRequest
			expectedFileName string
			expectingError   bool
		}{
			{
				name: "Success",
				input: &blobbergrpc.GetObjectTreeRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:       "/",
					Allocation: "",
				},
				expectedFileName: "root",
				expectingError:   false,
			},
			{
				name: "bad path",
				input: &blobbergrpc.GetObjectTreeRequest{
					Context: &blobbergrpc.RequestContext{
						Client:          "exampleOwnerId",
						ClientKey:       "",
						Allocation:      allocationTx,
						ClientSignature: clientSignature,
					},
					Path:       "/2",
					Allocation: "",
				},
				expectedFileName: "root",
				expectingError:   true,
			},
		}

		for _, tc := range testCases {

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

}
