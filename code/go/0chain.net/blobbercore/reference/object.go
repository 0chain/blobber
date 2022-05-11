package reference

import (
	"context"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

func DeleteObject(ctx context.Context, allocationID, objPath string) (*Ref, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Delete(&Ref{}, "allocation_id=? AND path LIKE ? AND path != ?",
		allocationID, objPath+"%", "/").Delete(&Ref{}).Error

	if err != nil {
		return nil, err
	}

	rootRef, err := GetReferencePath(ctx, allocationID, filepath.Dir(objPath))
	if err != nil {
		return nil, err
	}
	rootRef.HashToBeComputed = true
	return rootRef, nil
}
