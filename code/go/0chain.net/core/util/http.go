package util

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"

	"go.uber.org/zap"
)

const MAX_RETRIES = 5
const SLEEP_BETWEEN_RETRIES = 5

func NewHTTPRequest(method string, url string, data []byte) (req *http.Request, ctx context.Context, cncl context.CancelFunc, err error) {
	requestHash := encryption.Hash(data)
	req, err = http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("X-App-Client-ID", node.Self.ID)
	req.Header.Set("X-App-Client-Key", node.Self.PublicKey)
	req.Header.Set("X-App-Request-Hash", requestHash)
	ctx, cncl = context.WithTimeout(context.Background(), time.Second*10)
	return
}

func SendMultiPostRequest(urls []string, data []byte) {
	wg := sync.WaitGroup{}
	wg.Add(len(urls))

	for _, url := range urls {
		go SendPostRequest(url, data, &wg) //nolint:errcheck // goroutines
	}
	wg.Wait()
}

func SendPostRequest(url string, data []byte, wg *sync.WaitGroup) (body []byte, err error) {
	if wg != nil {
		defer wg.Done()
	}

	var (
		resp *http.Response
		req  *http.Request
		ctx  context.Context
		cncl context.CancelFunc
	)

	for i := 0; i < MAX_RETRIES; i++ {
		req, ctx, cncl, err = NewHTTPRequest(http.MethodPost, url, data)
		if err != nil {
			return nil, err
		}

		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				break
			}
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				err = common.NewError("read_error", err.Error())
			} else {
				err = common.NewError("http_error", "Error from HTTP call. "+string(body))
			}
			resp.Body.Close()
		}
		//TODO: Handle ctx cncl
		time.Sleep(SLEEP_BETWEEN_RETRIES * time.Second)
	}

	defer cncl()

	if resp == nil || err != nil {
		Logger.Error("Failed after multiple retries", zap.Any("url", url), zap.Int("retried", MAX_RETRIES), zap.Error(err))
		return nil, err
	}
	if resp.Body == nil {
		return nil, common.NewError("empty_body", "empty body returned")
	}

	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	return body, err
}
