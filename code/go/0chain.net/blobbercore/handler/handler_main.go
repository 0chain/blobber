//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	setupHandlers(r)

	r.HandleFunc("/v1/file/list/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))).
		Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/v1/file/upload/{allocation}", RateLimitByFileRL(common.ToJSONResponse(WithConnection(UploadHandler))))
	r.HandleFunc("/v1/file/download/{allocation}", RateLimitByFileRL(common.ToByteStream(WithConnection(DownloadHandler)))).Methods(http.MethodGet, http.MethodOptions)
}

// swagger:route GET /v1/block/magic/get getmagicblock
// a handler to respond to block queries
//
// responses:
//  200: ListResult
//  404:
/* ListHandler is the handler to respond to list requests from clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return listHandler(ctx, r)
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return downloadHandler(ctx, r)
}

func RedeemHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return redeemHandler(ctx, r)
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return uploadHandler(ctx, r)
}
