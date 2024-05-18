package reference

import (
	"context"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func DeleteObject(ctx context.Context, rootRef *Ref, allocationID, objPath string, ts common.Timestamp) error {
	likePath := objPath + "/%"
	if objPath == "/" {
		likePath = "/%"
	}

	db := datastore.GetStore().GetTransaction(ctx)

	err := db.Exec("UPDATE reference_objects SET is_precommit=? WHERE allocation_id=? AND path != ? AND (path=? OR path LIKE ?)", true, allocationID, "/", objPath, likePath).Error
	if err != nil {
		return err
	}

	err = db.Delete(&Ref{}, "allocation_id=? AND path != ? AND (path=? OR path LIKE ?)",
		allocationID, "/", objPath, likePath).Error

	if err != nil {
		return err
	}
	if objPath == "/" {
		rootRef.Children = nil
		rootRef.HashToBeComputed = true
		rootRef.childrenLoaded = true
		rootRef.UpdatedAt = ts
		return nil
	}
	parentPath, deleteFileName := filepath.Split(objPath)
	logging.Logger.Info("delete_object", zap.String("parent_path", parentPath), zap.String("delete_file_name", deleteFileName))
	rootRef.UpdatedAt = ts
	fields, err := common.GetPathFields(parentPath)
	if err != nil {
		return err
	}

	dirRef := rootRef

	for _, name := range fields {
		var found bool
		for _, ref := range dirRef.Children {
			if ref.Name == name {
				ref.HashToBeComputed = true
				ref.childrenLoaded = true
				ref.UpdatedAt = ts
				found = true
				dirRef = ref
				break
			}
		}

		if !found {
			return common.NewError("invalid_reference_path", "Reference path has invalid references")
		}
	}
	logging.Logger.Info("delete_object", zap.Any("dir_ref", dirRef))
	for i, child := range dirRef.Children {
		if child.Name == deleteFileName {
			dirRef.RemoveChild(i)
			break
		}
	}

	rootRef.HashToBeComputed = true
	rootRef.childrenLoaded = true
	return nil
}
