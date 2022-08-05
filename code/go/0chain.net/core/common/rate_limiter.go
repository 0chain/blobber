package common

import (
	"net/http"
	"strings"
	"time"

	tollbooth "github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/spf13/viper"
	rl "go.uber.org/ratelimit"
)

type ratelimit struct {
	Limiter           *limiter.Limiter
	RateLimit         bool
	RequestsPerSecond float64
}

var userRateLimit *ratelimit
var fileRateLimit *ratelimit

func initUserRateLimiter(userRl float64) {
	userRateLimit = &ratelimit{RequestsPerSecond: userRl}

	if userRateLimit.RequestsPerSecond == 0 {
		userRateLimit.RateLimit = false
		return
	}

	userRateLimit.RateLimit = true
	userRateLimit.Limiter = tollbooth.NewLimiter(userRateLimit.RequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour}).
		SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}).
		SetMethods([]string{"GET", "POST", "PUT", "DELETE"})
}

func initFileRateLimiter(fileRl float64) {
	fileRateLimit = &ratelimit{RequestsPerSecond: fileRl}

	if fileRateLimit.RequestsPerSecond == 0 {
		fileRateLimit.RateLimit = false
		return
	}

	fileRateLimit.RateLimit = true
	fileRateLimit.Limiter = tollbooth.NewLimiter(fileRateLimit.RequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour}).
		SetIPLookups([]string{}). // IP not required
		SetIgnoreURL(false)       // URL Path contains allocation
}

//ConfigRateLimits - configure the rate limits
func ConfigRateLimits() {
	initUserRateLimiter(viper.GetFloat64("handlers.rate_limit"))
	initFileRateLimiter(viper.GetFloat64("handlers.file_rate_limit"))
}

func NewGRPCRateLimiter() *GRPCRateLimiter {
	userRl := viper.GetFloat64("handlers.rate_limit")

	return &GRPCRateLimiter{rl.New(int(userRl))}
}

type GRPCRateLimiter struct {
	rl.Limiter
}

func (r *GRPCRateLimiter) Limit() bool {
	r.Take()
	return false
}

//UserRateLimit - rate limiting for end user handlers
func UserRateLimit(handler ReqRespHandlerf) ReqRespHandlerf {
	if !userRateLimit.RateLimit {
		return handler
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		tollbooth.LimitFuncHandler(userRateLimit.Limiter, handler).ServeHTTP(writer, request)
	}
}

//UseUserRateLimit - rate limiting for end user handlers
func UseUserRateLimit(h http.Handler) http.Handler {
	if !userRateLimit.RateLimit {
		return h
	}

	return tollbooth.LimitFuncHandler(userRateLimit.Limiter, h.ServeHTTP)
}

// FileRateLimit is a custom of `tollbooth.LimitHandlerFunc` that uses custom values to build keys for limiting file actions.
func FileRateLimit(handler ReqRespHandlerf) ReqRespHandlerf {
	if fileRateLimit == nil || !fileRateLimit.RateLimit {
		return handler
	}

	lmt := fileRateLimit.Limiter

	return func(w http.ResponseWriter, r *http.Request) {
		keys := FileLimitBuildKeys(lmt, r)
		httpError := tollbooth.LimitByKeys(lmt, keys)
		if httpError != nil {
			lmt.ExecOnLimitReached(w, r)
			if lmt.GetOverrideDefaultResponseWriter() {
				return
			}
			w.Header().Add("Content-Type", lmt.GetMessageContentType())
			w.WriteHeader(httpError.StatusCode)
			_, _ = w.Write([]byte(httpError.Message))
			return
		}

		// There's no rate-limit error, serve the next handler.
		handler(w, r)
	}
}

// FileLimitBuildKeys return build keys to be used for file rate limiter.
func FileLimitBuildKeys(lmt *limiter.Limiter, r *http.Request) []string {
	baseKeys := tollbooth.BuildKeys(lmt, r)

	client := r.Header.Get(ClientHeader)
	baseKeys[0] = append(baseKeys[0], strings.ToLower(client))

	// add connection_id when present (exists for upload)
	conn, ok := requestField(r, "connection_id")
	if ok {
		baseKeys[0] = append(baseKeys[0], strings.ToLower(conn))
		return baseKeys[0]
	}

	// OR add path hash (may exists for download)
	pathHash := r.Header.Get("X-Path-Hash")
	if pathHash != "" {
		baseKeys[0] = append(baseKeys[0], strings.ToLower(pathHash))
		return baseKeys[0]
	}

	// OR add path (may exists for download)
	path := r.Header.Get("X-Path")
	if path != "" {
		baseKeys[0] = append(baseKeys[0], path)
		return baseKeys[0]
	}

	// OR just empty
	baseKeys[0] = append(baseKeys[0], "")
	return baseKeys[0]
}

func requestField(r *http.Request, key string) (string, bool) {
	TryParseForm(r)

	if !r.Form.Has(key) {
		return "", false
	}

	return r.Form.Get(key), true
}
