package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateDir(t *testing.T) {
	list := []struct {
		TestName string

		ArgAllocationTx string
		ArgName         string

		Setup func()

		Result bool
		Error  error
	}{
		{TestName: "dir_not_exists", ArgAllocationTx: "TestCreateDir", ArgName: "/dir_not_exists", Result: true, Error: nil, Setup: func() {
			mock.MockRefNoExists("TestCreateDir", "/dir_not_exists")
		}},
		{TestName: "dir_exists", ArgAllocationTx: "TestCreateDir", ArgName: "/dir_exists", Result: true, Error: nil, Setup: func() {
			mock.MockRefExists("TestCreateDir", "/dir_exists")
		}},
	}

	datastore.UseMocket(true)

	for _, it := range list {
		t.Run(it.TestName, func(test *testing.T) {
			body := &bytes.Buffer{}
			formWriter := multipart.NewWriter(body)
			formWriter.WriteField("connection_id", "connection_id")
			formWriter.WriteField("name", it.ArgName)
			formWriter.Close()

			ctx := &Context{
				Store:        datastore.GetStore(),
				AllocationTx: it.ArgAllocationTx,
				Request:      httptest.NewRequest(http.MethodPut, "http://127.0.0.1/v1/dir/"+it.ArgAllocationTx, body),
			}

			ctx.Request.Header.Add("Content-Type", formWriter.FormDataContentType())

			it.Setup()

			result, err := CreateDir(ctx)

			r := require.New(test)

			r.Equal(it.Result, result)
			r.Equal(it.Error, err)

		})
	}
}
