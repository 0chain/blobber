package handler

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/0chain/gosdk/zboxcore/sdk"
)

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil
	}

	return b
}

func BenchmarkUploadFile(b *testing.B) {

	KB := 1024
	MB := 1024 * KB
	//GB := 1024 * MB

	datastore.UseMocket()

	formBuilder := sdk.CreateChunkedUploadFormBuilder()

	var storageHandler StorageHandler

	benchmarks := []struct {
		Name      string
		Size      int
		ChunkSize int
	}{
		{Name: "10M 64K", Size: MB * 10, ChunkSize: 64 * KB},
		{Name: "10M 6M", Size: MB * 10, ChunkSize: 6 * MB},
	}

	for _, bm := range benchmarks {
		b.Run(bm.Name, func(b *testing.B) {

			fileName := strings.Replace(bm.Name, " ", "_", -1) + ".txt"
			chunkBytes := generateRandomBytes(bm.ChunkSize)
			fileMeta := &sdk.FileMeta{
				Path:       "/tmp/" + fileName,
				ActualSize: int64(bm.Size),

				MimeType:   "plain/text",
				RemoteName: fileName,
				RemotePath: "/" + fileName,
				Attributes: fileref.Attributes{},
			}
			b.ResetTimer()

			hasher := sdk.CreateHasher(bm.ChunkSize)
			isFinal := false

			ctx := context.WithValue(context.TODO(), constants.ContextKeyClient, "client_id")
			ctx = context.WithValue(ctx, constants.ContextKeyClientKey, "client_key")
			ctx = context.WithValue(ctx, constants.ContextKeyAllocation, "allocation_id")

			ctx = GetMetaDataStore().CreateTransaction(ctx)

			for i := 0; i < b.N; i++ {
				body, _, _ := formBuilder.Build(fileMeta, hasher, "connectionID", int64(bm.ChunkSize), 0, isFinal, "", chunkBytes, nil)

				req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:5051//v1/file/upload/benchmark_upload", body)

				_, err := storageHandler.WriteFile(ctx, req)

				if err != nil {
					b.Fatal(err)
					return
				}

			}
		})
	}

}
