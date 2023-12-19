package common

import "net/http"

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
			r.ParseMultipartForm(FormFileParseMaxMemory) //nolint: errcheck
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
