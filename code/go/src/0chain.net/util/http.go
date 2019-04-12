package util

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"0chain.net/encryption"

	"0chain.net/common"
	. "0chain.net/logging"
	"0chain.net/node"

	"go.uber.org/zap"
)

const MAX_RETRIES = 5
const SLEEP_BETWEEN_RETRIES = 5

func NewHTTPRequest(method string, url string, data []byte) (*http.Request, context.Context, context.CancelFunc, error) {
	requestHash := encryption.Hash(data)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("X-App-Client-ID", node.Self.ID)
	req.Header.Set("X-App-Client-Key", node.Self.PublicKey)
	req.Header.Set("X-App-Request-Hash", requestHash)
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*10)
	return req, ctx, cncl, err
}

func SendMultiPostRequest(urls []string, data []byte) {
	wg := sync.WaitGroup{}
	wg.Add(len(urls))

	for _, url := range urls {
		go SendPostRequest(url, data, &wg)
	}
	wg.Wait()
}

func SendPostRequest(url string, data []byte, wg *sync.WaitGroup) ([]byte, error) {
	if wg != nil {
		defer wg.Done()
	}
	var resp *http.Response
	var err error
	for i := 0; i < MAX_RETRIES; i++ {
		req, ctx, cncl, err := NewHTTPRequest(http.MethodPost, url, data)
		defer cncl()
		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				break
			}
			body, _ := ioutil.ReadAll(resp.Body)
			if resp.Body != nil {
				resp.Body.Close()
			}
			err = common.NewError("http_error", "Error from HTTP call. "+string(body))
		}
		//TODO: Handle ctx cncl
		time.Sleep(SLEEP_BETWEEN_RETRIES * time.Second)
	}
	if resp == nil || err != nil {
		Logger.Error("Failed after multiple retries", zap.Any("url", url), zap.Int("retried", MAX_RETRIES))
		return nil, err
	}
	if resp.Body == nil {
		return nil, common.NewError("empty_body", "empty body returned")
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	body, _ := ioutil.ReadAll(resp.Body)
	return body, nil
}
