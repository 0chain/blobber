package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestWriteMarkerHandlers_Lock(t *testing.T) {
	config.Configuration.WriteMarkerLockTimeout = time.Second * 30
	datastore.UseMocket(true)

	r := mux.NewRouter()
	SetupHandlers(r)

	body := &bytes.Buffer{}
	formWriter := multipart.NewWriter(body)

	now := time.Now()

	formWriter.WriteField("connection_id", "connection_id")                  //nolint: errcheck
	formWriter.WriteField("request_time", strconv.FormatInt(now.Unix(), 10)) //nolint: errcheck
	formWriter.Close()

	req, err := http.NewRequest(http.MethodPost, "/v1/writemarker/lock/{allocation}", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", formWriter.FormDataContentType())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WithTxHandler(func(ctx *Context) (interface{}, error) {
		ctx.AllocationId = "TestHandlers_Lock_allocation_id"
		return LockWriteMarker(ctx)
	}))

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result writemarker.LockResult

	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.Nil(t, err)

	require.Equal(t, writemarker.LockStatusOK, result.Status)
}

func TestWriteMarkerHandlers_Unlock(t *testing.T) {
	datastore.UseMocket(true)

	r := mux.NewRouter()
	SetupHandlers(r)

	body := &bytes.Buffer{}
	formWriter := multipart.NewWriter(body)

	now := time.Now()

	formWriter.WriteField("connection_id", "connection_id")                  //nolint: errcheck
	formWriter.WriteField("request_time", strconv.FormatInt(now.Unix(), 10)) //nolint: errcheck
	formWriter.Close()

	req, err := http.NewRequest(http.MethodDelete, "/v1/writemarker/lock/{allocation}/{connection}", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", formWriter.FormDataContentType())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WithTxHandler(func(ctx *Context) (interface{}, error) {
		ctx.AllocationId = "TestHandlers_Unlock_allocation_id"
		ctx.Vars["connection"] = "connection_id"
		return UnlockWriteMarker(ctx)
	}))

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

}
