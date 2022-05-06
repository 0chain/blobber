package reference

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	_ "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeleteObject delete object from tree, and return tree root and deleted content hash list
func DeleteObject(ctx context.Context, allocationID, path string) (*Ref, map[string]bool, error) {
	rootRef, err := GetObjectTree(ctx, allocationID, "/")
	if err != nil {
		return nil, nil, err
	}

	rootRef.HashToBeComputed = true
	db := datastore.GetStore().
		GetTransaction(ctx)

	var deletedObjects []*Ref
	txDelete := db.Clauses(clause.Returning{Columns: []clause.Column{{Name: "content_hash"}, {Name: "thumbnail_hash"}, {Name: "type"}}})

	path = filepath.Join("/", path)
	txDelete = txDelete.Where("allocation_id = ? and deleted_at IS NULL and (path LIKE ? or path = ?) and path != ? ", allocationID, path+"%", path, "/")

	err = txDelete.Delete(&deletedObjects).Error
	if err != nil {
		return nil, nil, common.NewError("bad_db_operation", err.Error())
	}

	deletedFiles := make(map[string]bool)
	for _, it := range deletedObjects {
		if it.Type == FILE {
			deletedFiles[it.ContentHash] = true
			if it.ThumbnailSize > 0 {
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
				dirRef.HashToBeComputed = true
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
