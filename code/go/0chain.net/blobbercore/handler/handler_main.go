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


// swagger:route GET /v1/file/list/{allocation} list
// ListHandler is the handler to respond to list requests from clients, 
// it returns a list of files in the allocation,
// along with the metadata of the files.
//
// parameters:
//
//   +name: allocation
//     description: TxHash of the allocation in question.
//     in: path
//     required: true
//     type: string
//   +name: list
//     description: Whether or not to list the files inside the directory, not just data about the path itself.
//     in: query
//     type: boolean
//     required: false
//   +name: limit
//	   description: The number of files to return (for pagination).
//     in: query
//     type: integer
//     required: true
//   +name: offset
//     description: The number of files to skip before returning (for pagination).
//     in: query
//     type: integer
//     required: true
//	 +name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: ALLOCATION-ID
//	   description: The ID of the allocation in question.
//     in: header
//     type: string
//     required: true
//
// responses:
//
//   200: ListResult

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
