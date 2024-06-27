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

	s := r.NewRoute().Subrouter()
	setupHandlers(s)

	s.HandleFunc("/v1/file/list/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))).
		Methods(http.MethodGet, http.MethodOptions)
	s.HandleFunc("/v1/file/upload/{allocation}", RateLimitByFileRL(common.ToJSONResponse(WithConnection(UploadHandler))))
	s.HandleFunc("/v1/file/download/{allocation}", RateLimitByFileRL(common.ToByteStream(WithConnection(DownloadHandler)))).Methods(http.MethodGet, http.MethodOptions)
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
