package reference

import (
	"context"
	"database/sql"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func DeleteObject(ctx context.Context, allocationID, objPath string, ts common.Timestamp) error {
	likePath := objPath + "/%"
	if objPath == "/" {
		likePath = "/%"
	}

	db := datastore.GetStore().GetTransaction(ctx)

	err := db.Exec("UPDATE reference_objects SET is_precommit=?,deleted_at=? WHERE allocation_id=? AND path != ? AND (path=? OR path LIKE ?)", true, sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}, allocationID, "/", objPath, likePath).Error
	if err != nil {
		return err
	}
	return err
}
