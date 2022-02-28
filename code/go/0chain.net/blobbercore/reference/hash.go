package reference

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"gorm.io/gorm"
)

// LoadRootNode load root node with its descendant nodes
func LoadRootNode(ctx context.Context, allocationID string) (*HashNode, error) {

	db := datastore.GetStore().GetDB()

	db = db.Raw(`
SELECT allocation_id, type, name, path, content_hash, merkle_root, actual_file_hash, attributes, chunk_size,size,actual_file_size, parent_path
FROM reference_objects
WHERE allocation_id = ? and deleted_at IS NULL
ORDER BY level desc, path`, allocationID)

	rows, err := db.Rows()
	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	defer rows.Close()

	nodes := make(map[string]*HashNode)
	for rows.Next() {

		node := &HashNode{}
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
		return &HashNode{AllocationID: allocationID, Type: DIRECTORY, Path: "/", Name: "/", ParentPath: ""}, nil
	}

	root, ok := nodes["/"]

	if ok {
		return root, nil
	}

	return nil, common.ErrMissingRootNode
}

const (
	SQLWhereGetByAllocationTxAndPath = "reference_objects.allocation_id = ? and reference_objects.path = ? and deleted_at is NULL"
)

// DryRun  Creates a prepared statement when executing any SQL and caches them to speed up future calls
// https://gorm.io/docs/performance.html#Caches-Prepared-Statement
func DryRun(db *gorm.DB) {

	// https://gorm.io/docs/session.html#DryRun
	// Session mode
	//tx := db.Session(&gorm.Session{PrepareStmt: true, DryRun: true})

	// use Table instead of Model to reduce reflect times
}
