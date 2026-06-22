package handler

import (
	"net/http"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// diskUsedPctFn measures disk fullness. Overridable in tests.
var diskUsedPctFn = func() (int, error) {
	return filestore.GetDiskUsedPct(config.Configuration.MountPoint)
}

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
				logging.Logger.Error("[recover]http", zap.String("url", escapedUrl), zap.Any("err", err))
			}
		}()

		h.ServeHTTP(w, r)
	})
}

// WithDiskSpaceCheck wraps an upload handler and returns 507 Insufficient Storage
// when the blobber mount-point disk is at or above config.DiskWriteThreshold percent
// full. A threshold of 0 disables the check. On stat errors the check fails open
// (the upload proceeds) so a transient stat failure never blocks writes.
func WithDiskSpaceCheck(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threshold := config.Configuration.DiskWriteThreshold
		if threshold > 0 {
			pct, err := diskUsedPctFn()
			if err != nil {
				logging.Logger.Error("disk space check failed, allowing write",
					zap.Error(err))
			} else if pct >= threshold {
				logging.Logger.Warn("upload rejected: disk above write threshold",
					zap.Int("used_pct", pct),
					zap.Int("threshold", threshold))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInsufficientStorage)
				_, _ = w.Write([]byte(`{"error":"disk_full","description":"blobber storage is above write threshold"}`))
				return
			}
		}
		next(w, r)
	}
}
