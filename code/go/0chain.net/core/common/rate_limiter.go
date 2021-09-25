package common

import (
	"net/http"
	"time"

	"github.com/didip/tollbooth/v6"
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

func (rl *ratelimit) init() {
	if rl.RequestsPerSecond == 0 {
		rl.RateLimit = false
		return
	}
	rl.RateLimit = true
	rl.Limiter = tollbooth.NewLimiter(rl.RequestsPerSecond, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour}).
		SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}).
		SetMethods([]string{"GET", "POST", "PUT", "DELETE"})
}

//ConfigRateLimits - configure the rate limits
func ConfigRateLimits() {
	userRl := viper.GetFloat64("handlers.rate_limit")

	userRateLimit = &ratelimit{RequestsPerSecond: userRl}
	userRateLimit.init()
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
func UserRateLimit(h http.Handler) http.Handler {
	if !userRateLimit.RateLimit {
		return h
	}

	return tollbooth.LimitFuncHandler(userRateLimit.Limiter, h.ServeHTTP)
}
