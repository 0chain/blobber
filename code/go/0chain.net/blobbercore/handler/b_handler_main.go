//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"net/http"
)

/*ListHandler is the handler to respond to list requests from clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return listHandler(ctx, r)
}
