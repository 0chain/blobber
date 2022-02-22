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

	db = db.Where("allocation_id = ? and deleted_at IS NULL", allocationID)

	db = db.Order("level desc, path")

	dict := make(map[string][]*HashNode)

	var nodes []*HashNode
	// it is better to load them in batched if there are a lot of objects in db
	err := db.FindInBatches(&nodes, 100, func(tx *gorm.DB, batch int) error {
		// batch processing found records
		for _, object := range nodes {
			dict[object.ParentPath] = append(dict[object.ParentPath], object)

			for _, child := range dict[object.Path] {
				object.AddChild(child)
			}
		}

		return nil
	}).Error

	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	// create empty dir if root is missing
	if len(dict) == 0 {
		return &HashNode{AllocationID: allocationID, Type: DIRECTORY, Path: "/", Name: "/", ParentPath: ""}, nil
	}

	rootNodes, ok := dict[""]

	if ok {
		if len(rootNodes) == 1 {
			return rootNodes[0], nil
		}

		return nil, errors.Throw(common.ErrInternal, "invalid_ref_tree: / is missing or invalid")
	}

	return nil, errors.Throw(common.ErrInternal, "invalid_ref_tree: / is missing or invalid")
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
