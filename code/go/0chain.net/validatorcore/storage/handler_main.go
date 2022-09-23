//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
)

/*setupHandlers sets up the necessary API end points */
func setupHandlers(r *mux.Router) {
	r.Use(common.UseUserRateLimit)
	r.HandleFunc("/v1/storage/challenge/new", common.ToJSONResponse(SetupContext(ChallengeHandler)))
	r.HandleFunc("/debug", common.ToJSONResponse(DumpGoRoutines))
}
