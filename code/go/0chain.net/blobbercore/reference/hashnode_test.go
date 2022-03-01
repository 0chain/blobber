package reference

import (
	"context"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	gomocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
)

<<<<<<< HEAD
func TestHashnode_Should_Work(t *testing.T) {
=======
func TestHashabelNode_Should_Work(t *testing.T) {
>>>>>>> 2d6b112358bc723fdba885604cfd3cb2ba8a68e9

	datastore.UseMocket(true)

	tests := []struct {
		name         string
		allocationID string
		mock         func()
		assert       func(*testing.T, string, *Hashnode, error)
	}{
		{
			name:         "No any node should work",
			allocationID: "allocation_none",
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).WithArgs("allocation_none").
					WithReply(nil)
			},
			assert: func(test *testing.T, allocationID string, r *Hashnode, err error) {
				require.NotNil(test, r)
				require.Len(test, r.Children, 0)

				require.Equal(test, allocationID, r.AllocationID)
				require.Equal(test, DIRECTORY, r.Type)
				require.Equal(test, "/", r.Name)
				require.Equal(test, "/", r.Path)
				require.Equal(test, "", r.ContentHash)
				require.Equal(test, "", r.MerkleRoot)
				require.Equal(test, "", r.ActualFileHash)
				require.EqualValues(test, 0, r.ChunkSize)
				require.EqualValues(test, 0, r.Size)
				require.EqualValues(test, 0, r.ActualFileSize)

				buf, e := r.Attributes.MarshalJSON() //nolint
				require.Nil(test, e)

				require.Equal(test, "null", string(buf))

				require.Equal(test, "", r.ParentPath)

			},
		},
		{
			name:         "Nested node should work",
			allocationID: "allocation_nested",
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).
					WithArgs("allocation_nested").
					WithReply([]map[string]interface{}{
						{
							"allocation_id":    "allocation_nested",
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
							"allocation_id":    "allocation_nested",
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
							"allocation_id":    "allocation_nested",
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
							"allocation_id":    "allocation_nested",
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

			},
			assert: func(test *testing.T, allocationID string, r *Hashnode, err error) {
				require.NotNil(test, r)
				require.Len(test, r.Children, 2)

				require.Equal(test, r.Children[0].Name, "sub1")
				require.Len(test, r.Children[0].Children, 1)
				require.Equal(test, r.Children[0].Children[0].Name, "file1")
				require.Equal(test, r.Children[1].Name, "sub2")

			},
		},
	}

	for _, it := range tests {

		t.Run(it.name,
			func(test *testing.T) {
				if it.mock != nil {
					it.mock()
				}

				r, err := LoadRootHashnode(context.TODO(), it.allocationID)

				it.assert(test, it.allocationID, r, err)

			},
		)

	}

}

<<<<<<< HEAD
func TestHashnode_Should_Not_Work(t *testing.T) {
=======
func TestHashabelNode_Should_Not_Work(t *testing.T) {
>>>>>>> 2d6b112358bc723fdba885604cfd3cb2ba8a68e9

	datastore.UseMocket(true)

	tests := []struct {
		name         string
		allocationID string
		mock         func()
		assert       func(*testing.T, string, *Hashnode, error)
	}{
		{
			name:         "Missing root node should not work",
			allocationID: "allocation_missing",
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).WithArgs("allocation_missing").
					WithReply([]map[string]interface{}{
						{
							"allocation_id":    "allocation_missing",
							"type":             "D",
							"name":             "sub",
							"path":             "/sub",
							"content_hash":     "",
							"merkle_root":      "",
							"actual_file_hash": "",
							"attributes":       []byte("null"),
							"chunk_size":       0,
							"size":             0,
							"actual_file_size": 0,
							"parent_path":      "/",
						}})
			},
			assert: func(test *testing.T, allocationID string, r *Hashnode, err error) {
				require.Nil(test, r)
				require.ErrorIs(test, common.ErrMissingRootNode, err)

			},
		},
		{
			name:         "Duplicated root node should not work",
			allocationID: "allocation_duplicated_root",
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).
					WithArgs("allocation_duplicated_root").
					WithReply([]map[string]interface{}{
						{
							"allocation_id":    "allocation_duplicated_root",
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
							"allocation_id":    "allocation_duplicated_root",
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
							"allocation_id":    "allocation_duplicated_root",
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
							"allocation_id":    "allocation_duplicated_root",
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
							"allocation_id":    "allocation_duplicated_root",
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

			},
			assert: func(test *testing.T, allocationID string, r *Hashnode, err error) {
				require.Nil(test, r)
				require.ErrorIs(test, common.ErrDuplicatedNode, err)

			},
		},
		{
			name:         "Duplicated node should not work",
			allocationID: "allocation_duplicated_node",
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects`).
					WithArgs("allocation_duplicated_node").
					WithReply([]map[string]interface{}{
						{
							"allocation_id":    "allocation_duplicated_node",
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
							"allocation_id":    "allocation_duplicated_node",
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
							"allocation_id":    "allocation_duplicated_node",
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
							"allocation_id":    "allocation_duplicated_node",
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
						{
							"allocation_id":    "allocation_duplicated_node",
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

			},
			assert: func(test *testing.T, allocationID string, r *Hashnode, err error) {
				require.Nil(test, r)
				require.ErrorIs(test, common.ErrDuplicatedNode, err)

			},
		},
	}

	for _, it := range tests {

		t.Run(it.name,
			func(test *testing.T) {
				if it.mock != nil {
					it.mock()
				}

				r, err := LoadRootHashnode(context.TODO(), it.allocationID)

				it.assert(test, it.allocationID, r, err)

			},
		)

	}

}
