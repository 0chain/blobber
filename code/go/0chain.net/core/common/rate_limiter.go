package common

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	tollbooth "github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
)

func GetRateLimiter(rps float64, ipLookups []string, ignoreUrl bool, tokExpireTTL time.Duration) *limiter.Limiter {
	lmt := tollbooth.NewLimiter(rps, &limiter.ExpirableOptions{
		DefaultExpirationTTL: tokExpireTTL,
	})

	if ipLookups != nil {
		lmt.SetIPLookups(ipLookups)
	}

	lmt.SetIgnoreURL(ignoreUrl)
	return lmt
}

func RateLimit(handler ReqRespHandlerf, lmt *limiter.Limiter) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		TryParseForm(r)

		if lmt != nil {

			keys := tollbooth.BuildKeys(lmt, r)
			clientID := r.Header.Get(ClientHeader)

			keys = append(keys, []string{clientID})

			for _, k := range keys {
				httpError := tollbooth.LimitByKeys(lmt, k)
				if httpError != nil {
					logging.Logger.Error(fmt.Sprintf("Rate limit error: %s", httpError.Error()))
					lmt.ExecOnLimitReached(w, r)
					setResponseHeaders(lmt, w, r)
					w.Header().Add("Content-Type", lmt.GetMessageContentType())
					w.WriteHeader(httpError.StatusCode)
					w.Write([]byte(httpError.Message)) // nolint
					return
				}
			}
		}
		handler(w, r)
	}
}

func RateLimitByIP(handler ReqRespHandlerf, lmt *limiter.Limiter) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		TryParseForm(r)

		if lmt != nil {

			keys := tollbooth.BuildKeys(lmt, r)
			for _, k := range keys {
				httpError := tollbooth.LimitByKeys(lmt, k)
				if httpError != nil {
					logging.Logger.Error(fmt.Sprintf("Rate limit error: %s", httpError.Error()))
					lmt.ExecOnLimitReached(w, r)
					setResponseHeaders(lmt, w, r)
					w.Header().Add("Content-Type", lmt.GetMessageContentType())
					w.WriteHeader(httpError.StatusCode)
					w.Write([]byte(httpError.Message)) // nolint
					return
				}
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
