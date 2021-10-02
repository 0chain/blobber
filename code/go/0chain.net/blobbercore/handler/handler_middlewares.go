package handler

import (
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/handlers"
	"go.uber.org/zap"
)

func useCORS() func(http.Handler) http.Handler {
	headersOk := handlers.AllowedHeaders([]string{
		"X-Requested-With", "X-App-Client-ID",
		"X-App-Client-Key", "Content-Type",
		"X-App-Client-Signature",
	})

	// Allow anybody to access API.
	// originsOk := handlers.AllowedOriginValidator(isValidOrigin)
	originsOk := handlers.AllowedOrigins([]string{"*"})

	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT",
		"DELETE", "OPTIONS"})

	return handlers.CORS(originsOk, headersOk, methodsOk)
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
