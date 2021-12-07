package handler

import (
	"net/http"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func useCors(h http.Handler) http.Handler {
	allowedHeaders := []string{
		"X-Requested-With", "Content-Type",
		"X-App-Client-ID", "X-App-Client-Key", "X-App-Client-Signature",
		"Access-Control-Allow-origin", "Access-Control-Request-Method",
	}

	allowedOrigins := []string{"*"}

	allowedMethods := []string{"GET", "HEAD", "POST", "PUT",
		"DELETE", "OPTIONS"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logging.Logger.Error("[recover]http", zap.String("url", r.URL.String()), zap.Any("err", err))
			}
		}()

		w.Header().Add("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
		w.Header().Add("Access-Control-Allow-Origin", strings.Join(allowedOrigins, ", "))
		w.Header().Add("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
		w.Header().Add("Access-Control-Allow-Credentials", "true")

		// return directly for preflight request
		if r.Method == http.MethodOptions {
			w.Header().Add("Access-Control-Max-Age", "3600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func useRecovery(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logging.Logger.Error("[recover]http", zap.String("url", r.URL.String()), zap.Any("err", err))
			}
		}()

		h.ServeHTTP(w, r)
	})
}
