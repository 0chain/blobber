package storage

import (
	"context"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

const (
	RequestPerSecond      = 10
	DefualtExpirationTime = time.Minute * 5
)

var lmt *limiter.Limiter

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	ConfigureRateLimiter()
	r.HandleFunc("/v1/storage/challenge/new",
		RateLimit(common.ToJSONResponse(SetupContext(ChallengeHandler))))

	r.HandleFunc("/debug", common.ToJSONResponse(DumpGoRoutines))
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {
	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}

func ConfigureRateLimiter() {
	rps := viper.GetFloat64("rate_limiters.request_per_second")
	if rps <= 0 {
		rps = RequestPerSecond
	}

	tokenExpirettl := viper.GetDuration("rate_limiters.default_token_expire_duration")
	if tokenExpirettl <= 0 {
		tokenExpirettl = DefualtExpirationTime
	}

	isProxy := viper.GetBool("rate_limiters.proxy")
	ipLookups := []string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}

	if isProxy {
		ipLookups = []string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"}
	}

	lmt = common.GetRateLimiter(rps, ipLookups, true, tokenExpirettl)
}

func RateLimit(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return common.RateLimit(handler, lmt)
}
