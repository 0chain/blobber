package handler

import (
	"net/http"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func UseCors(h http.Handler) http.Handler {

	allowedMethods := []string{"GET", "HEAD", "POST", "PUT",
		"DELETE", "OPTIONS"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				escapedUrl := sanitizeString(r.URL.String())
				logging.Logger.Error("[recover]http", zap.String("url", escapedUrl), zap.Any("err", err))
			}
		}()

		w.Header().Add("Access-Control-Allow-Headers", "*")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))

		// return directly for preflight request
		if r.Method == http.MethodOptions {
			w.Header().Add("Access-Control-Max-Age", "3600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func UseRecovery(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				escapedUrl := sanitizeString(r.URL.String())
				logging.Logger.Error("[recover]http", zap.String("url", escapedUrl), zap.Any("err", err), zap.Stack("recover_stack"))
			}
		}()

		h.ServeHTTP(w, r)
	})
}
