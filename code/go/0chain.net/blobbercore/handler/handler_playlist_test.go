package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/gorilla/mux"
	gomocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
)

func TestPlaylist_LoadPlaylist(t *testing.T) {

	datastore.UseMocket(true)

	gomocket.Catcher.NewMock().
		WithQuery(`SELECT "lookup_hash","name","path","num_of_blocks","parent_path","size","mimetype","type" FROM "reference_objects" WHERE allocation_id = $1 and parent_path = $2 and type='f' and name like '%!!(string=path)t(string=)s`).
		WithArgs("AllocationId", "path").
		WithReply([]map[string]interface{}{
			{
				"lookup_hash":   "lookup_hash1",
				"name":          "name1",
				"path":          "path1",
				"num_of_blocks": 1,
				"parent_path":   "parent_path1",
				"size":          10,
				"mimetype":      "mimetype1",
				"type":          "f",
			},
			{
				"lookup_hash":   "lookup_hash2",
				"name":          "name2",
				"path":          "path2",
				"num_of_blocks": 2,
				"parent_path":   "parent_path2",
				"size":          20,
				"mimetype":      "mimetype2",
				"type":          "f",
			},
		})

	r := mux.NewRouter()
	SetupHandlers(r)

	req, err := http.NewRequest(http.MethodGet, "/v1/playlist/latest/{allocation}?path=path", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WithTxHandler(func(ctx *Context) (interface{}, error) {
		ctx.AllocationId = "AllocationId"
		ctx.ClientID = "ownerid"
		ctx.Allocation = &allocation.Allocation{
			ID:      "AllocationId",
			OwnerID: "ownerid",
		}
		return LoadPlaylist(ctx)
	}))

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var files []reference.PlaylistFile

	err = json.Unmarshal(rr.Body.Bytes(), &files)
	require.Nil(t, err)

	require.NotNil(t, files)
	require.Len(t, files, 2)

	require.Equal(t, files[0].LookupHash, "lookup_hash1")
	require.Equal(t, files[0].Name, "name1")
	require.Equal(t, files[0].Path, "path1")
	require.Equal(t, files[0].NumBlocks, int64(1))
	require.Equal(t, files[0].ParentPath, "parent_path1")
	require.Equal(t, files[0].Size, int64(10))
	require.Equal(t, files[0].MimeType, "mimetype1")
	require.Equal(t, files[0].Type, "f")

	require.Equal(t, files[1].LookupHash, "lookup_hash2")
	require.Equal(t, files[1].Name, "name2")
	require.Equal(t, files[1].Path, "path2")
	require.Equal(t, files[1].NumBlocks, int64(2))
	require.Equal(t, files[1].ParentPath, "parent_path2")
	require.Equal(t, files[1].Size, int64(20))
	require.Equal(t, files[1].MimeType, "mimetype2")
	require.Equal(t, files[1].Type, "f")
}

func TestPlaylist_LoadPlaylistFile(t *testing.T) {

	datastore.UseMocket(true)

	gomocket.Catcher.NewMock().
		WithQuery(`SELECT "lookup_hash","name","path","num_of_blocks","parent_path","size","mimetype","type" FROM "reference_objects" WHERE allocation_id = $1 and lookup_hash = $2 ORDER BY "reference_objects"."lookup_hash" LIMIT 1`).
		WithArgs("AllocationId", "lookup_hash").
		WithReply([]map[string]interface{}{
			{
				"lookup_hash":   "lookup_hash",
				"name":          "name",
				"path":          "path",
				"num_of_blocks": 1,
				"parent_path":   "parent_path",
				"size":          10,
				"mimetype":      "mimetype",
				"type":          "f",
			},
		})

	r := mux.NewRouter()
	SetupHandlers(r)

	req, err := http.NewRequest(http.MethodGet, "/v1/playlist/file/{allocation}?lookup_hash=lookup_hash", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WithTxHandler(func(ctx *Context) (interface{}, error) {
		ctx.AllocationId = "AllocationId"
		ctx.ClientID = "ownerid"
		ctx.Allocation = &allocation.Allocation{
			ID:      "AllocationId",
			OwnerID: "ownerid",
		}
		return LoadPlaylistFile(ctx)
	}))

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	file := &reference.PlaylistFile{}

	err = json.Unmarshal(rr.Body.Bytes(), file)
	require.Nil(t, err)

	require.NotNil(t, file)

	require.Equal(t, file.LookupHash, "lookup_hash")
	require.Equal(t, file.Name, "name")
	require.Equal(t, file.Path, "path")
	require.Equal(t, file.NumBlocks, int64(1))
	require.Equal(t, file.ParentPath, "parent_path")
	require.Equal(t, file.Size, int64(10))
	require.Equal(t, file.MimeType, "mimetype")
	require.Equal(t, file.Type, "f")
}
