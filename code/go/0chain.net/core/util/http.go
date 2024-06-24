package util

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"

	"go.uber.org/zap"
)

const MAX_RETRIES = 3
const SLEEP_BETWEEN_RETRIES = 5

func NewHTTPRequest(method, url string, data []byte) (*http.Request, context.Context, context.CancelFunc, error) {
	requestHash := encryption.Hash(data)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("X-App-Client-ID", node.Self.ID)
	req.Header.Set("X-App-Client-Key", node.Self.PublicKey)
	req.Header.Set("X-App-Request-Hash", requestHash)
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*90)
	return req, ctx, cncl, err
}

func SendPostRequest(postURL string, data []byte, wg *sync.WaitGroup) (body []byte, err error) {
	if wg != nil {
		defer wg.Done()
	}
	var resp *http.Response
	u, err := url.Parse(postURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	for i := 0; i < MAX_RETRIES; i++ {
		var (
			req  *http.Request
			ctx  context.Context
			cncl context.CancelFunc
		)

		req, ctx, cncl, err = NewHTTPRequest(http.MethodPost, u.String(), data)
		defer cncl()
		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				break
			}
			body, _ = io.ReadAll(resp.Body)
			if resp.Body != nil {
				resp.Body.Close()
			}
			err = common.NewError("http_error", "Error from HTTP call. "+string(body))
		}
		//TODO: Handle ctx cncl
		time.Sleep(SLEEP_BETWEEN_RETRIES * time.Second)
	}
	if resp == nil || err != nil {
		Logger.Error("Failed after multiple retries", zap.Any("url", u.String()), zap.Int("retried", MAX_RETRIES), zap.Int("post_data_len", len(data)), zap.Error(err))
		return nil, err
	}
	if resp.Body == nil {
		return nil, common.NewError("empty_body", "empty body returned")
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	body, err = io.ReadAll(resp.Body)
	return body, err
}
