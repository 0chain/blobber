package reference

import (
	"context"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func DeleteObject(ctx context.Context, allocationID, objPath string, ts common.Timestamp) (*Ref, error) {

	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Delete(&Ref{}, "allocation_id=? AND path != ? AND (path=? OR path LIKE ?)",
		allocationID, "/", objPath, objPath+"/%").Error

	if err != nil {
		return nil, err
	}

	parentPath := filepath.Dir(objPath)
	rootRef, err := GetReferencePath(ctx, allocationID, parentPath)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	fields, err := common.GetPathFields(parentPath)
	if err != nil {
		return nil, err
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
			return nil, common.NewError("invalid_reference_path", "Reference path has invalid references")
		}
	}

	rootRef.HashToBeComputed = true
	rootRef.childrenLoaded = true
	return rootRef, nil
}
