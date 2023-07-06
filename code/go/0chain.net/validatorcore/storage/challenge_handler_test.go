//go:build !integration_tests
// +build !integration_tests

package storage_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/validatorcore/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestChallengeHandler(t *testing.T) {
	logging.Logger = zap.New(nil) // FIXME to avoid complains
	tests := []struct {
		name       string
		ctx        context.Context
		req        *http.Request
		want       interface{}
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "wrong request method",
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "url", bytes.NewBuffer([]byte("{}")))
				return req
			}(),
			wantErr:    true,
			wantErrMsg: "Invalid method used for the upload URL. Use multi-part form POST instead",
		},
		{
			name: "invalid body",
			req: func() *http.Request {
				req, _ := http.NewRequest("POST", "url", bytes.NewBuffer([]byte("body")))
				return req
			}(),
			wantErr:    true,
			wantErrMsg: "Error in decoding the input.",
		},
		{
			name: "hash mismatch",
			req: func() *http.Request {
				req, _ := http.NewRequest("POST", "url", bytes.NewBuffer([]byte("{}")))
				req.Header.Set("X-App-Request-Hash", "840eb7aa2a9935de63366bacbe9d97e978a859e93dc792a0334de60ed52f8e90")
				return req
			}(),
			wantErr:    true,
			wantErrMsg: "Header hash and request hash do not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			if tt.ctx != nil {
				ctx = tt.ctx
			}
			got, err := storage.ChallengeHandler(ctx, tt.req)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
