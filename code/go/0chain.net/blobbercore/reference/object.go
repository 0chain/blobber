package reference

import (
	"context"
	"database/sql"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func DeleteObject(ctx context.Context, allocationID, lookupHash string, ts common.Timestamp) error {
	db := datastore.GetStore().GetTransaction(ctx)

	err := db.Exec("UPDATE reference_objects SET is_precommit=?,deleted_at=? WHERE lookup_hash=?", true, sql.NullTime{
		Time:  common.ToTime(ts),
		Valid: true,
	}, lookupHash).Error
	if err != nil {
		logging.Logger.Error("delete_object_error", zap.Error(err))
		return err
	}
	return err
}
