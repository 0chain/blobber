package common

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestFileRateLimit(t *testing.T) {
	initFileRateLimiter(1)
	defer initFileRateLimiter(0)

	uploadReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/v1/file/upload/alloc123", nil)
	require.Nil(t, err)

	limitMiddleware := FileRateLimit(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	recorder := httptest.NewRecorder()
	limitMiddleware(recorder, uploadReq)
	assert.Equal(t, 200, recorder.Code)

	recorder = httptest.NewRecorder()
	limitMiddleware(recorder, uploadReq)
	assert.Equal(t, 429, recorder.Code) // rate limiter should kick in

	<-time.After(1 * time.Second)
	recorder = httptest.NewRecorder()
	limitMiddleware(recorder, uploadReq)
	assert.Equal(t, 200, recorder.Code)
}

func TestFileLimitBuildKeys(t *testing.T) {
	initFileRateLimiter(1)
	defer initFileRateLimiter(0)

	for _, tc := range []struct {
		name      string
		req       *http.Request
		reqHeader map[string]string
		reqForm   map[string]string
		wantKeys  []string
	}{
		{
			name:      "upload",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/v1/file/upload/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient"},
			reqForm:   map[string]string{"connection_id": "abcdef"},
			wantKeys:  []string{"", "/v1/file/upload/alloc123", "", "testclient", "abcdef"},
		},
		{
			name:      "upload with empty client",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/v1/file/upload/alloc123", nil)),
			reqHeader: map[string]string{},
			reqForm:   map[string]string{"connection_id": "abcdef"},
			wantKeys:  []string{"", "/v1/file/upload/alloc123", "", "", "abcdef"},
		},
		{
			name:      "upload with empty connection_id",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/v1/file/upload/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient"},
			reqForm:   map[string]string{"connection_id": ""},
			wantKeys:  []string{"", "/v1/file/upload/alloc123", "", "testclient", ""},
		},
		{
			name:      "upload with connection_id in multipart",
			req:       multipartReq(context.Background(), http.MethodPost, "http://localhost:8080/v1/file/upload/alloc123", map[string]string{"connection_id": "abcdef"}),
			reqHeader: map[string]string{ClientHeader: "testclient"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/upload/alloc123", "", "testclient", "abcdef"},
		},
		{
			name:      "download with path hash",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/v1/file/download/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient", "X-Path-Hash": "hash"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/download/alloc123", "", "testclient", "hash"},
		},
		{
			name:      "download with path",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/v1/file/download/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient", "X-Path": "path"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/download/alloc123", "", "testclient", "path"},
		},
		{
			name:      "download with path and hash",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/v1/file/download/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient", "X-Path-Hash": "hash", "X-Path": "path"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/download/alloc123", "", "testclient", "hash"},
		},
		{
			name:      "download without path nor hash",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/v1/file/download/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "testclient"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/download/alloc123", "", "testclient", ""},
		},
		{
			name:      "download without client",
			req:       mustReq(http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/v1/file/download/alloc123", nil)),
			reqHeader: map[string]string{ClientHeader: "", "X-Path-Hash": "hash"},
			reqForm:   map[string]string{},
			wantKeys:  []string{"", "/v1/file/download/alloc123", "", "", "hash"},
		},
	} {
		tt := tc
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.reqHeader {
				tt.req.Header.Add(k, v)
			}

			if tt.req.Form == nil {
				tt.req.Form = url.Values{}
			}

			for k, v := range tt.reqForm {
				tt.req.Form.Add(k, v)
			}

			got := FileLimitBuildKeys(fileRateLimit.Limiter, tt.req)

			assert.Equal(t, tt.wantKeys, got)
		})
	}
}

func mustReq(r *http.Request, _ error) *http.Request {
	return r
}

func multipartReq(ctx context.Context, method, url string, multiPart map[string]string) *http.Request {
	body := &bytes.Buffer{}

	formWriter := multipart.NewWriter(body)

	for k, v := range multiPart {
		formWriter.WriteField(k, v)
	}

	formWriter.Close()

	r := mustReq(http.NewRequestWithContext(ctx, method, url, body))
	r.Header.Add("Content-Type", formWriter.FormDataContentType())

	return r
}
