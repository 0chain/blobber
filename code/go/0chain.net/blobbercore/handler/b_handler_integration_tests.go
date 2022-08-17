//go:build integration_tests
// +build integration_tests

package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

/*ListHandler is the handler to respond to list requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	state := conductrpc.Client().State()

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.SendWrongData {
		var result blobberhttp.ListResult
		return result, nil
	}

	return listHandler(ctx, r)
}
