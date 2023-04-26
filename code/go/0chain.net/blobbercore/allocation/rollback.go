package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"gorm.io/gorm"
)

func ApplyRollback(ctx context.Context, allocationID string) error {

	db := datastore.GetStore().GetTransaction(ctx)

	// delete all is_temp rows

	err := db.Transaction(func(tx *gorm.DB) error {
		err := db.Model(&reference.Ref{}).Unscoped().
			Delete(&reference.Ref{},
				"allocation_id=? AND is_temp=?",
				allocationID, true).Error
		if err != nil {
			return err
		}

		// revive soft deleted rows
		err = db.Exec("UPDATE reference_objects SET deleted_at=NULL WHERE allocation_id=? AND deleted_at IS NOT NULL", allocationID).Error

		if err != nil {
			return err
		}
		return nil
	})

	return err
}

func CommitRollback(allocationID string) error {

	err := filestore.GetFileStore().DeletePreCommitDir(allocationID)
	return err
}
