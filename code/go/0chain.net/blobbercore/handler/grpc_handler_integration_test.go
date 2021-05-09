package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"0chain.net/blobbercore/config"
	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
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
		t.Skip()
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

		getAllocationReq := &blobbergrpc.GetAllocationRequest{
			Context: &blobbergrpc.RequestContext{},
			Id:      "exampleTransaction",
		}

		getAllocationResp, err := blobberClient.GetAllocation(ctx, getAllocationReq)
		if err != nil {
			t.Fatal(err)
		}

		if getAllocationResp.Allocation.Tx != getAllocationReq.Id {
			t.Fatal("unexpected allocation id from GetAllocation rpc")
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

		req := &blobbergrpc.GetFileMetaDataRequest{
			Context: &blobbergrpc.RequestContext{
				Client: "exampleOwnerId",
			},
			Path:       "examplePath",
			PathHash:   "exampleId:examplePath",
			Allocation: "exampleTransaction",
		}
		getFileMetaDataResp, err := blobberClient.GetFileMetaData(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if getFileMetaDataResp.MetaData.FileMetaData.Name != "filename" {
			t.Fatal("unexpected file name from GetFileMetaData rpc")
		}
	})

	t.Run("TestGetFileStats", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetFileStatsTestData()
		if err != nil {
			t.Fatal(err)
		}

		req := &blobbergrpc.GetFileStatsRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "exampleOwnerId",
				ClientKey:  "",
				Allocation: "exampleTransaction",
			},
			Path:     "examplePath",
			PathHash: "exampleId:examplePath",
		}

		getFileStatsResp, err := blobberClient.GetFileStats(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if getFileStatsResp.MetaData.FileMetaData.Name != "filename" {
			t.Fatal("unexpected file name from GetFileStats rpc")
		}
	})

	t.Run("TestListEntities", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddListEntitiesTestData()
		if err != nil {
			t.Fatal(err)
		}

		req := &blobbergrpc.ListEntitiesRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "exampleOwnerId",
				ClientKey:  "",
				Allocation: "exampleTransaction",
			},
			Path:       "examplePath",
			PathHash:   "exampleId:examplePath",
			AuthToken:  "",
			Allocation: "",
		}

		listEntitiesResp, err := blobberClient.ListEntities(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if listEntitiesResp.MetaData.DirMetaData.Path != "examplePath" {
			t.Fatal("unexpected path from ListEntities rpc")
		}
	})

	t.Run("TestGetObjectPath", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetObjectPathTestData()
		if err != nil {
			t.Fatal(err)
		}

		req := &blobbergrpc.GetObjectPathRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "exampleOwnerId",
				ClientKey:  "",
				Allocation: "exampleTransaction",
			},
			Allocation: "",
			Path:       "examplePath",
			BlockNum:   "0",
		}

		getObjectPathResp, err := blobberClient.GetObjectPath(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if getObjectPathResp.ObjectPath.Path.DirMetaData.Path != "/" {
			t.Fatal("unexpected root hash from GetObjectPath rpc")
		}
	})

	t.Run("TestGetReferencePath", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetReferencePathTestData()
		if err != nil {
			t.Fatal(err)
		}

		req := &blobbergrpc.GetReferencePathRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "exampleOwnerId",
				ClientKey:  "",
				Allocation: "exampleTransaction",
			},
			Paths:      "",
			Path:       "/",
			Allocation: "",
		}
		getReferencePathResp, err := blobberClient.GetReferencePath(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if getReferencePathResp.ReferencePath.MetaData.DirMetaData.Path != "/" {
			t.Fatal("unexpected path from GetReferencePath rpc")
		}
	})

	t.Run("TestGetObjectTree", func(t *testing.T) {
		err := tdController.ClearDatabase()
		if err != nil {
			t.Fatal(err)
		}
		err = tdController.AddGetObjectTreeTestData()
		if err != nil {
			t.Fatal(err)
		}

		req := &blobbergrpc.GetObjectTreeRequest{
			Context: &blobbergrpc.RequestContext{
				Client:     "exampleOwnerId",
				ClientKey:  "",
				Allocation: "exampleTransaction",
			},
			Path:       "/",
			Allocation: "",
		}
		getObjectTreeResp, err := blobberClient.GetObjectTree(ctx, req)
		if err != nil {
			t.Fatal(err)
		}

		if getObjectTreeResp.ReferencePath.MetaData.DirMetaData.Name != "root" {
			t.Fatal("unexpected root name from GetObject")
		}
	})

}
