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
}

/* ListHandler is the handler to respond to list requests from clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return listHandler(ctx, r)
}
