package handler

import (
	"net/http"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func checkValidDate(s, dateLayOut string) error {
	if s != "" {
		_, err := time.Parse(dateLayOut, s)
		if err != nil {
			return common.NewError("invalid_parameters", err.Error())
		}
	}
	return nil
}

// TryParseForm try populates r.Form and r.PostForm.
func TryParseForm(r *http.Request) {
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		ct := r.Header.Get("Content-Type")
		if ct == "application/x-www-form-urlencoded" {
			r.ParseForm() //nolint: errcheck
		} else {
			r.ParseMultipartForm(FormFileParseMaxMemory) //nolint: errcheck
		}
	}
}
