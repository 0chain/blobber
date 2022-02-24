package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mock"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/0chain/gosdk/zboxcore/sdk"
)

func BenchmarkUploadFileWithDisk(b *testing.B) {
	KB := 1024
	MB := 1024 * KB
	//GB := 1024 * MB

	datastore.UseMocket(false)
	blobber := mock.NewBlobberClient()

	allocationID := "benchmark_uploadfile"

	allocation := map[string]interface{}{
		"id":               allocationID,
		"tx":               allocationID,
		"size":             1024 * 1024 * 100,
		"blobber_size":     1024 * 1024 * 1000,
		"owner_id":         blobber.ClientID,
		"owner_public_key": blobber.Wallet.Keys[0].PublicKey,
		"expiration_date":  time.Now().Add(24 * time.Hour).Unix(),
	}

	mock.MockGetAllocationByID(allocationID, allocation)

	formBuilder := sdk.CreateChunkedUploadFormBuilder()

	var storageHandler StorageHandler

	benchmarks := []struct {
		Name string
		// Size      int
		ChunkSize int
	}{
		{Name: "64K", ChunkSize: 64 * KB},
		{Name: "640K", ChunkSize: 640 * KB},
		{Name: "6M", ChunkSize: 6 * MB},
		{Name: "60M", ChunkSize: 60 * MB},
	}

	for _, bm := range benchmarks {
		b.Run(bm.Name, func(b *testing.B) {
			fileName := strings.Replace(bm.Name, " ", "_", -1) + ".txt"
			chunkBytes := mock.GenerateRandomBytes(bm.ChunkSize)
			fileMeta := &sdk.FileMeta{
				Path:       "/tmp/" + fileName,
				ActualSize: int64(bm.ChunkSize),

				MimeType:   "plain/text",
				RemoteName: fileName,
				RemotePath: "/" + fileName,
				Attributes: fileref.Attributes{},
			}

			hasher := sdk.CreateHasher(bm.ChunkSize)
			isFinal := false

			body, formData, _ := formBuilder.Build(fileMeta, hasher, strconv.FormatInt(time.Now().UnixNano(), 10), int64(bm.ChunkSize), 0, isFinal, "", chunkBytes, nil)

			req, err := blobber.NewRequest(http.MethodPost, "http://127.0.0.1:5051/v1/file/upload/benchmark_upload", body)

			if err != nil {
				b.Fatal(err)
				return
			}

			req.Header.Set("Content-Type", formData.ContentType)
			err = blobber.SignRequest(req, allocationID)
			if err != nil {
				b.Fatal(err)
				return
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := GetMetaDataStore().CreateTransaction(context.TODO())
				ctx = mock.SetupHandlerContext(ctx, req, allocationID)
				_, err := storageHandler.WriteFile(ctx, req)

				if err != nil {
					b.Fatal(err)
					return
				}
			}
		})
	}
}

func BenchmarkUploadFileWithNoDisk(b *testing.B) {
	KB := 1024
	MB := 1024 * KB
	//GB := 1024 * MB

	datastore.UseMocket(false)
	filestore.UseMock(nil)
	blobber := mock.NewBlobberClient()

	allocationID := "benchmark_uploadfile"

	allocation := map[string]interface{}{
		"id":               allocationID,
		"tx":               allocationID,
		"size":             1024 * 1024 * 100,
		"blobber_size":     1024 * 1024 * 1000,
		"owner_id":         blobber.ClientID,
		"owner_public_key": blobber.Wallet.Keys[0].PublicKey,
		"expiration_date":  time.Now().Add(24 * time.Hour).Unix(),
	}

	mock.MockGetAllocationByID(allocationID, allocation)

	formBuilder := sdk.CreateChunkedUploadFormBuilder()

	var storageHandler StorageHandler

	benchmarks := []struct {
		Name string
		//	Size      int
		ChunkSize int
	}{
		{Name: "64K", ChunkSize: 64 * KB},
		{Name: "640K", ChunkSize: 640 * KB},
		{Name: "6M", ChunkSize: 6 * MB},
		{Name: "60M", ChunkSize: 60 * MB},
	}

	for _, bm := range benchmarks {
		b.Run(bm.Name, func(b *testing.B) {
			fileName := strings.Replace(bm.Name, " ", "_", -1) + ".txt"
			chunkBytes := mock.GenerateRandomBytes(bm.ChunkSize)
			fileMeta := &sdk.FileMeta{
				Path:       "/tmp/" + fileName,
				ActualSize: int64(bm.ChunkSize),

				MimeType:   "plain/text",
				RemoteName: fileName,
				RemotePath: "/" + fileName,
				Attributes: fileref.Attributes{},
			}

			hasher := sdk.CreateHasher(bm.ChunkSize)
			isFinal := false

			body, formData, _ := formBuilder.Build(fileMeta, hasher, strconv.FormatInt(time.Now().UnixNano(), 10), int64(bm.ChunkSize), 0, isFinal, "", chunkBytes, nil)

			req, err := blobber.NewRequest(http.MethodPost, "http://127.0.0.1:5051/v1/file/upload/benchmark_upload", body)

			if err != nil {
				b.Fatal(err)
				return
			}

			req.Header.Set("Content-Type", formData.ContentType)
			err = blobber.SignRequest(req, allocationID)
			if err != nil {
				b.Fatal(err)
				return
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := GetMetaDataStore().CreateTransaction(context.TODO())

				ctx = mock.SetupHandlerContext(ctx, req, allocationID)
				_, err := storageHandler.WriteFile(ctx, req)

				if err != nil {
					b.Fatal(err)
					return
				}
			}
		})
	}
}
