package storage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

const (
	RequestPerSecond      = 5
	DefualtExpirationTime = time.Minute * 5
)

var lmt *limiter.Limiter

func init() {
	defaultIPLookups := []string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}
	lmt = tollbooth.NewLimiter(RequestPerSecond, &limiter.ExpirableOptions{
		DefaultExpirationTTL: DefualtExpirationTime,
	})

	lmt.SetIgnoreURL(true)
	lmt.SetIPLookups(defaultIPLookups)
}

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
	tokenExpirettl := viper.GetDuration("rate_limiters.default_token_expire_duration")
	isProxy := viper.GetBool("rate_limiters.proxy")

	if tokenExpirettl > 0 {
		lmt.SetTokenBucketExpirationTTL(tokenExpirettl)
	}

	if isProxy {
		ipLookup := []string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"}
		lmt.SetIPLookups(ipLookup)
	}
}

func RateLimit(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error())) // nolint
			return
		}

		httpError := tollbooth.LimitByRequest(lmt, w, r)
		if httpError != nil {
			lmt.ExecOnLimitReached(w, r)
			setResponseHeaders(lmt, w, r)
			w.Header().Add("Content-Type", lmt.GetMessageContentType())
			w.WriteHeader(httpError.StatusCode)
			w.Write([]byte(httpError.Message)) // nolint
			return
		}
		handler(w, r)
	}
}

func setResponseHeaders(lmt *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Rate-Limit-Limit", fmt.Sprintf("%.2f", lmt.GetMax()))
	w.Header().Add("X-Rate-Limit-Duration", "1")

	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if strings.TrimSpace(xForwardedFor) != "" {
		w.Header().Add("X-Rate-Limit-Request-Forwarded-For", xForwardedFor)
	}

	w.Header().Add("X-Rate-Limit-Request-Remote-Addr", r.RemoteAddr)
}
