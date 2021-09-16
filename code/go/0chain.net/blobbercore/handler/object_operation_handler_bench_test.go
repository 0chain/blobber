package handler

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	formBuilder := sdk.CreateChunkedUploadFormBuilder()

	// setupHandlerContext := func(ctx context.Context, r *http.Request) context.Context {
	// 	var vars = mux.Vars(r)
	// 	ctx = context.WithValue(ctx, constants.ContextKeyClient,
	// 		r.Header.Get(common.ClientHeader))
	// 	ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
	// 		r.Header.Get(common.ClientKeyHeader))
	// 	ctx = context.WithValue(ctx, constants.ContextKeyAllocation,
	// 		vars["allocation"])
	// 	// signature is not required for all requests, but if header is empty it won`t affect anything
	// 	ctx = context.WithValue(ctx, constants.ContextKeyClientSignatureHeaderKey, r.Header.Get(common.ClientSignatureHeader))
	// 	return ctx
	// }

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
			fileBytes := generateRandomBytes(bm.Size)
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

			for i := 0; i < b.N; i++ {
				body, _, _ := formBuilder.Build(fileMeta, hasher, "connectionID", int64(bm.ChunkSize), i, isFinal, "", fileBytes, nil)

				req := httptest.NewRequest(http.MethodPost, "", body)

				_, err := storageHandler.WriteFile(context.TODO(), req)

				if err != nil {
					b.Fatal(err)
					return
				}

			}
		})
	}

}
