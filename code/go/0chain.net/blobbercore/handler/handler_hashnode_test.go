//go:build !integration
// +build !integration

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/gorilla/mux"
	gomocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
)

func TestHashnodeHanders_LoadRootHashnode(t *testing.T) {

	datastore.UseMocket(true)

	gomocket.Catcher.NewMock().
		WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).
		WithArgs("allocation_handler_load_root").
		WithReply([]map[string]interface{}{
			{
				"allocation_id":    "allocation_handler_load_root",
				"type":             "D",
				"name":             "/",
				"path":             "/",
				"content_hash":     "",
				"merkle_root":      "",
				"actual_file_hash": "",
				"attributes":       []byte("null"),
				"chunk_size":       0,
				"size":             0,
				"actual_file_size": 0,
				"parent_path":      "",
			},
			{
				"allocation_id":    "allocation_handler_load_root",
				"type":             "D",
				"name":             "sub1",
				"path":             "/sub1",
				"content_hash":     "",
				"merkle_root":      "",
				"actual_file_hash": "",
				"attributes":       []byte("null"),
				"chunk_size":       0,
				"size":             0,
				"actual_file_size": 0,
				"parent_path":      "/",
			},
			{
				"allocation_id":    "allocation_handler_load_root",
				"type":             "D",
				"name":             "sub2",
				"path":             "/sub2",
				"content_hash":     "",
				"merkle_root":      "",
				"actual_file_hash": "",
				"attributes":       []byte("null"),
				"chunk_size":       0,
				"size":             0,
				"actual_file_size": 0,
				"parent_path":      "/",
			},
			{
				"allocation_id":    "allocation_handler_load_root",
				"type":             "D",
				"name":             "file1",
				"path":             "/sub1/file1",
				"content_hash":     "",
				"merkle_root":      "",
				"actual_file_hash": "",
				"attributes":       []byte("null"),
				"chunk_size":       0,
				"size":             0,
				"actual_file_size": 0,
				"parent_path":      "/sub1",
			},
		})

	r := mux.NewRouter()
	SetupHandlers(r)

	req, err := http.NewRequest(http.MethodGet, "/v1/refs/root/{allocation}", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WithHandler(func(ctx *Context) (interface{}, error) {
		ctx.AllocationTx = "allocation_handler_load_root"
		return LoadRootHashnode(ctx)
	}))

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var root reference.Hashnode

	err = json.Unmarshal(rr.Body.Bytes(), &root)
	require.Nil(t, err)

	require.NotNil(t, root)
	require.Len(t, root.Children, 2)

	require.Equal(t, root.Children[0].Name, "sub1")
	require.Len(t, root.Children[0].Children, 1)
	require.Equal(t, root.Children[0].Children[0].Name, "file1")
	require.Equal(t, root.Children[1].Name, "sub2")
}
