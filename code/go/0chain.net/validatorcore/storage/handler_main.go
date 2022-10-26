//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	ConfigureRateLimiter()
	r.HandleFunc("/v1/storage/challenge/new",
		RateLimit(common.ToJSONResponse(SetupContext(ChallengeHandler))))

	r.HandleFunc("/debug", common.ToJSONResponse(DumpGoRoutines))
}
