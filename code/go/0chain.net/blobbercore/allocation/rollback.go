package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

func ApplyRollback(ctx context.Context, allocationID string) error {

	db := datastore.GetStore().GetTransaction(ctx)

	// delete all is_precommit rows

	err := db.Model(&reference.Ref{}).Unscoped().
		Delete(&reference.Ref{},
			"allocation_id=? AND is_precommit=? AND deleted_at IS NULL",
			allocationID, true).Error
	if err != nil {
		return err
	}

	// err = db.Exec("UPDATE file_stats SET deleted_at=NULL WHERE ref_id IN (SELECT id FROM reference_objects WHERE allocation_id=? AND deleted_at IS NOT NULL)", allocationID).Error
	// revive soft deleted ref rows
	err = db.Exec("UPDATE reference_objects SET deleted_at=NULL,is_precommit=? WHERE allocation_id=? AND deleted_at IS NOT NULL", false, allocationID).Error
	return err
}

func ApplyRollbackV2(ctx context.Context, allocationID, allocationRoot string, timestamp int64) error {
	db := datastore.GetStore().GetTransaction(ctx)

	err := db.Model(&reference.Ref{}).Unscoped().
		Delete(&reference.Ref{},
			"allocation_id=? AND allocation_root=? AND created_at=? AND deleted_at is NULL",
			allocationID, allocationRoot, timestamp).Error
	if err != nil {
		return err
	}

	return db.Exec("UPDATE reference_objects SET deleted_at=NULL WHERE allocation_id=? AND deleted_at IS NOT NULL", allocationID).Error
}

func CommitRollback(allocationID string) error {

	err := filestore.GetFileStore().DeletePreCommitDir(allocationID)
	return err
}
