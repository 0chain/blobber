package reference

import (
	"context"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LoadObjectTree
func LoadObjectTree(ctx context.Context, allocationID, path string) (*Ref, error) {
	db := datastore.GetStore().
		GetTransaction(ctx)

	if strings.HasSuffix(path, "/") {
		db = db.Where("allocation_id = ? and deleted_at IS NULL and path LIKE ? ", (path + "%"), allocationID)
	} else {
		db = db.Where("allocation_id = ? and deleted_at IS NULL and path LIKE ? ", (path + "/%"), allocationID)
	}

	db = db.Order("level desc, path")

	obejctTreeNodes := make(map[string][]*Ref)

	var objects []*Ref
	// it is better to load them in batched if there are a lot of objects in db
	err := db.FindInBatches(&objects, 100, func(tx *gorm.DB, batch int) error {
		// batch processing found records
		for _, object := range objects {
			parent, ok := obejctTreeNodes[object.ParentPath]
			if !ok {
				parent = make([]*Ref, 0)
			}

			obejctTreeNodes[object.ParentPath] = append(parent, object)

			child, ok := obejctTreeNodes[object.Path]
			if ok {
				object.Children = child
			}

		}

		return nil
	}).Error

	if err != nil {
		return nil, common.NewError("bad_db_operation", err.Error())
	}

	// create empty dir if root is missing
	if len(obejctTreeNodes) == 0 {
		return &Ref{Type: DIRECTORY, Path: "/", Name: "/", ParentPath: "", PathLevel: 1}, nil
	}

	rootNodes, ok := obejctTreeNodes[""]

	if ok {
		if len(rootNodes) == 1 {
			return rootNodes[0], nil
		}

		return nil, common.NewError("invalid_ref_tree", "/ is missing or invalid")

	}

	return nil, common.NewError("invalid_ref_tree", "/ is missing")

}

// DeleteObject delete object from tree, and return tree root and deleted content hash list
func DeleteObject(ctx context.Context, allocationID string, path string) (*Ref, map[string]bool, error) {

	db := datastore.GetStore().
		GetTransaction(ctx)

	var deletedObjects []*Ref
	txDelete := db.Clauses(clause.Returning{Columns: []clause.Column{{Name: "content_hash"}, {Name: "type"}}})

	if strings.HasSuffix(path, "/") {
		txDelete = txDelete.Where("allocation_id = ? and deleted_at IS NULL and path LIKE ? ", (path + "%"), allocationID)
	} else {
		txDelete = txDelete.Where("allocation_id = ? and deleted_at IS NULL and path LIKE ? ", (path + "/%"), allocationID)
	}

	err := txDelete.Delete(&deletedObjects).Error

	if err != nil {
		return nil, nil, common.NewError("bad_db_operation", err.Error())
	}

	deletedFiles := make(map[string]bool)
	for _, it := range deletedObjects {
		if it.Type == FILE {
			deletedFiles[it.ContentHash] = true
		}
	}

	rootRef, err := LoadObjectTree(ctx, allocationID, "/")

	if err != nil {
		return nil, nil, err
	}

	return rootRef, deletedFiles, nil
}
