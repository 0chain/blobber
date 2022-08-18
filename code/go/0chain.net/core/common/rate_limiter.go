package common

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	tollbooth "github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/spf13/viper"
)

type RPS float64

const (
	LimitByCommitRL  RPS = iota + 1 // Limit by commit rate limiter
	LimitByFileRL                   // Limit by file rate limiter
	LimitByObjectRL                 // Limit by object rate limiter
	LimitByGeneralRL                // Limit by general rate limiter
)

const (
	CommitRPS  = 0.5 // Commit Request Per Second
	FileRPS    = 1   // File Request Per Second
	ObjectRPS  = 0.5 // Object Request Per Second
	GeneralRPS = 5   // General Request Per Second
)

var (
	commitRL  *limiter.Limiter // commit Rate Limiter
	fileRL    *limiter.Limiter // file Rate Limiter
	objectRL  *limiter.Limiter // object Rate Limiter
	generalRL *limiter.Limiter // general Rate Limiter
)

func init() {
	defaultIPLookups := []string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}
	commitRL = tollbooth.NewLimiter(CommitRPS, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Minute * 5,
	})
	commitRL.SetIgnoreURL(true)
	commitRL.SetIPLookups(defaultIPLookups)

	fileRL = tollbooth.NewLimiter(FileRPS, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Minute * 5,
	})
	fileRL.SetIgnoreURL(true)
	fileRL.SetIPLookups(defaultIPLookups)

	objectRL = tollbooth.NewLimiter(ObjectRPS, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Minute * 5,
	})
	objectRL.SetIgnoreURL(true)
	objectRL.SetIPLookups(defaultIPLookups)

	generalRL = tollbooth.NewLimiter(GeneralRPS, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Minute * 5,
	})
	generalRL.SetIgnoreURL(true)
	generalRL.SetIPLookups(defaultIPLookups)
}

//ConfigRateLimits - configure the rate limits
func ConfigRateLimits() {
	tokenExpirettl := viper.GetDuration("rate_limiters.default_token_expire_duration")
	isProxy := viper.GetBool("rate_limiters.proxy")

	if tokenExpirettl > 0 {
		commitRL.SetTokenBucketExpirationTTL(tokenExpirettl)
		fileRL.SetTokenBucketExpirationTTL(tokenExpirettl)
		objectRL.SetTokenBucketExpirationTTL(tokenExpirettl)
		generalRL.SetTokenBucketExpirationTTL(tokenExpirettl)

	}

	if isProxy {
		// If blobber is behind some proxy then it is important to put
		// "X-Forwarded-For" in fron of other parameters.
		ipLookup := []string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"}
		commitRL.SetIPLookups(ipLookup)
		fileRL.SetIPLookups(ipLookup)
		objectRL.SetIPLookups(ipLookup)
		generalRL.SetIPLookups(ipLookup)
	}

	cRps := viper.GetFloat64("rate_limiters.commit_rps")
	fRps := viper.GetFloat64("rate_limiters.file_rps")
	oRps := viper.GetFloat64("rate_limiters.object_rps")
	gRps := viper.GetFloat64("rate_limiters.general_rps")

	if cRps > 0 {
		commitRL.SetMax(cRps)
	}

	if fRps > 0 {
		fileRL.SetMax(fRps)
	}

	if oRps > 0 {
		objectRL.SetMax(oRps)
	}

	if gRps > 0 {
		generalRL.SetMax(gRps)
	}
}

func RateLimit(handler ReqRespHandlerf, rlType RPS) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		TryParseForm(r)

		var lmt *limiter.Limiter
		switch rlType {
		case LimitByCommitRL:
			lmt = commitRL
		case LimitByFileRL:
			lmt = fileRL
		case LimitByObjectRL:
			lmt = objectRL
		case LimitByGeneralRL:
			lmt = generalRL
		}

		if lmt != nil {

			keys := tollbooth.BuildKeys(lmt, r)
			clientID := r.Header.Get(ClientHeader)

			keys[0] = append(keys[0], clientID)

			httpError := tollbooth.LimitByKeys(lmt, keys[0])
			if httpError != nil {
				lmt.ExecOnLimitReached(w, r)
				setResponseHeaders(lmt, w, r)
				w.Header().Add("Content-Type", lmt.GetMessageContentType())
				w.WriteHeader(httpError.StatusCode)
				w.Write([]byte(httpError.Message)) // nolint
				return
			}
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
