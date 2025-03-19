package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

func ApplyRollback(ctx context.Context, allocationID string, allocationVersion int64) error {

	db := datastore.GetStore().GetTransaction(ctx)

	// delete all current allocation version rows

	err := db.Model(&reference.Ref{}).Unscoped().
		Delete(&reference.Ref{},
			"allocation_id=? AND allocation_version=? AND deleted_at IS NULL",
			allocationID, allocationVersion).Error
	if err != nil {
		return err
	}

	// err = db.Exec("UPDATE file_stats SET deleted_at=NULL WHERE ref_id IN (SELECT id FROM reference_objects WHERE allocation_id=? AND deleted_at IS NOT NULL)", allocationID).Error
	// revive soft deleted ref rows
	err = db.Exec("UPDATE reference_objects SET deleted_at=NULL WHERE allocation_id=? AND deleted_at IS NOT NULL", allocationID).Error
	return err
}

func CommitRollback(allocationID string) error {

	err := filestore.GetFileStore().DeletePreCommitDir(allocationID)
	return err
}
