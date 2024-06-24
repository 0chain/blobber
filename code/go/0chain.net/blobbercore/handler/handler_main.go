//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
	"net/http"
	"sync"
)

var (
	BlobberRegisterMutex sync.Mutex
	BlobberRegisterCond  = sync.NewCond(&BlobberRegisterMutex)
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/_blobber_info", common.ToJSONResponse(GetBlobberInfo))

	// Wait for registration to complete
	go func() {
		BlobberRegisterMutex.Lock()
		defer BlobberRegisterMutex.Unlock()
		BlobberRegisterCond.Wait()

		// Setup the remaining handlers after registration completes
		setupHandlers(r)

		r.HandleFunc("/v1/file/list/{allocation}",
			RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))).
			Methods(http.MethodGet, http.MethodOptions)
		r.HandleFunc("/v1/file/upload/{allocation}", RateLimitByFileRL(common.ToJSONResponse(WithConnection(UploadHandler))))
		r.HandleFunc("/v1/file/download/{allocation}", RateLimitByFileRL(common.ToByteStream(WithConnection(DownloadHandler)))).Methods(http.MethodGet, http.MethodOptions)
	}()
}

func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return listHandler(ctx, r)
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return downloadHandler(ctx, r)
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return uploadHandler(ctx, r)
}
