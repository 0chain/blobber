//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
	"net/http"
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/_blobber_info", RateLimitByCommmitRL(common.ToJSONResponse(GetBlobberInfo)))
	setupHandlers(r)
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
