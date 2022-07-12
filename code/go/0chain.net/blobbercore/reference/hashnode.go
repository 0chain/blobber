package reference

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
)

// LoadRootHashnode load root node with its descendant nodes
func LoadRootHashnode(ctx context.Context, allocationID string) (*Hashnode, error) {

	db := datastore.GetStore().GetDB()

	db = db.Raw(`
SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, chunk_size,size,actual_file_size, parent_path
FROM reference_objects
WHERE allocation_id = ?
ORDER BY level desc, path`, allocationID)

	rows, err := db.Rows()
	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	defer rows.Close()

	nodes := make(map[string]*Hashnode)
	for rows.Next() {

		node := &Hashnode{}
		err = db.ScanRows(rows, node)
		if err != nil {
			return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
		}

		_, ok := nodes[node.Path]
		if ok {
			return nil, common.ErrDuplicatedNode
		}

		nodes[node.Path] = node

		parent, ok := nodes[node.ParentPath]
		if ok {
			parent.AddChild(node)
		}

	}

	// create empty dir if root is missing
	if len(nodes) == 0 {
		return &Hashnode{AllocationID: allocationID, Type: DIRECTORY, Path: "/", Name: "/", ParentPath: ""}, nil
	}

	root, ok := nodes["/"]

	if ok {
		return root, nil
	}

	return nil, common.ErrMissingRootNode
}
