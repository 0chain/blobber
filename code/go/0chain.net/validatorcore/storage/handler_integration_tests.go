//go:build integration_tests
// +build integration_tests

package storage

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
	"github.com/gorilla/mux"
)

/*setupHandlers sets up the necessary API end points */
func setupHandlers(r *mux.Router) {
	r.Use(common.UseUserRateLimit)
	r.HandleFunc("/v1/storage/challenge/new", common.ToJSONOrNotResponse(setupContextNotRespond(ChallengeHandler)))
	r.HandleFunc("/debug", common.ToJSONResponse(DumpGoRoutines))
}

func setupContextNotRespond(handler common.JSONResponderOrNotF) common.JSONResponderOrNotF {
	return func(ctx context.Context, r *http.Request) (interface{}, error, bool) {
		ctx = context.WithValue(ctx, constants.ContextKeyClient, r.Header.Get(common.ClientHeader))
		ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
			r.Header.Get(common.ClientKeyHeader))
		res, err, shouldRespond := handler(ctx, r)
		return res, err, shouldRespond
	}
}
