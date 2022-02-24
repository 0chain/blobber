package reference

import (
	"context"
	"path/filepath"
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

	path = filepath.Join("/", path)
	db = db.Where("allocation_id = ? and deleted_at IS NULL and path LIKE ? ", allocationID, (path + "%"))

	db = db.Order("level desc, lookup_hash")

	obejctTreeNodes := make(map[string][]*Ref)

	var objects []*Ref
	// it is better to load them in batched if there are a lot of objects in db
	err := db.FindInBatches(&objects, 100, func(tx *gorm.DB, batch int) error {
		// batch processing found records
		for _, object := range objects {
			obejctTreeNodes[object.ParentPath] = append(obejctTreeNodes[object.ParentPath], object)

			for _, child := range obejctTreeNodes[object.Path] {
				object.AddChild(child)
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
func DeleteObject(ctx context.Context, allocationID, path string) (*Ref, map[string]bool, error) {
	rootRef, err := LoadObjectTree(ctx, allocationID, "/")
	if err != nil {
		return nil, nil, err
	}

	db := datastore.GetStore().
		GetTransaction(ctx)

	var deletedObjects []*Ref
	txDelete := db.Clauses(clause.Returning{Columns: []clause.Column{{Name: "content_hash"}, {Name: "thumbnail_hash"}, {Name: "type"}}})

	path = filepath.Join("/", path)
	txDelete = txDelete.Where("allocation_id = ? and deleted_at IS NULL and (path LIKE ? or path = ?) and path != ? ", allocationID, (path + "%"), path, "/")

	err = txDelete.Delete(&deletedObjects).Error
	if err != nil {
		return nil, nil, common.NewError("bad_db_operation", err.Error())
	}

	deletedFiles := make(map[string]bool)
	for _, it := range deletedObjects {
		if it.Type == FILE {
			deletedFiles[it.ContentHash] = true
			if it.ThumbnailHash != "" {
				deletedFiles[it.ThumbnailHash] = true
			}
		}
	}

	// remove deleted object from tree
	if path == "/" {
		rootRef.Children = nil
		return rootRef, deletedFiles, nil
	}

	path = strings.TrimSuffix(path, "/")
	tSubDirs := GetSubDirsFromPath(path)

	dirRef := rootRef
	treelevel := 0
	for treelevel < len(tSubDirs)-1 {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == tSubDirs[treelevel] && child.Type == DIRECTORY {
				dirRef = child
				found = true
				break
			}
		}
		if !found {
			return nil, nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
		treelevel++
	}

	for i, child := range dirRef.Children {
		if child.Path == path {
			dirRef.RemoveChild(i)
			return rootRef, deletedFiles, nil
		}
	}
	return nil, nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
}
