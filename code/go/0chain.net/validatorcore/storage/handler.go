package storage

import (
	"context"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/gorilla/mux"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/storage/challenge/new", common.UserRateLimit(common.ToJSONResponse(SetupContext(ChallengeHandler))))
	r.HandleFunc("/debug", common.UserRateLimit(common.ToJSONResponse(DumpGoRoutines)))
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {
	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}
