package common

import (
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

const (
	FormFileParseMaxMemory = 32 * 1024 * 1024
)

// TryParseForm try populates r.Form and r.PostForm.
func TryParseForm(r *http.Request) {
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		ct := r.Header.Get("Content-Type")
		if ct == "application/x-www-form-urlencoded" {
			r.ParseForm() //nolint: errcheck
		} else {
			err := r.ParseMultipartForm(FormFileParseMaxMemory) //nolint: errcheck
			if err != nil {
				logging.Logger.Error("TryParseForm: ParseMultipartForm", zap.Error(err))
			}
		}
	}
}

// GetField get field from form or query
func GetField(r *http.Request, key string) (string, bool) {
	TryParseForm(r)

	v, ok := r.Form[key]
	if ok {
		return v[0], true
	}

	v, ok = r.URL.Query()[key]
	if ok {
		return v[0], true
	}

	return "", false
}
