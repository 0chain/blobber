package storage

import (
	"context"
	"github.com/0chain/common/constants/endpoint/v1_endpoint/validator_endpoint"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/gorilla/mux"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.Use(common.UseUserRateLimit)
	r.HandleFunc(validator_endpoint.NewChallenge.Path(), common.ToJSONResponse(SetupContext(ChallengeHandler)))
	r.HandleFunc(validator_endpoint.Debug.Path(), common.ToJSONResponse(DumpGoRoutines))
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {
	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}
