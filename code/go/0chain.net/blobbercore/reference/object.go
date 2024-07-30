package reference

import (
	"context"
	"database/sql"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func DeleteObject(ctx context.Context, allocationID, lookupHash, _type string, ts common.Timestamp, allocationVersion int64, collector QueryCollector) error {
	db := datastore.GetStore().GetTransaction(ctx)
	if _type == DIRECTORY {
		ref, err := GetLimitedRefFieldsByLookupHashWith(ctx, allocationID, lookupHash, []string{"id", "type"})
		if err != nil {
			logging.Logger.Error("delete_object_error", zap.Error(err))
			return err
		}
		isEmpty, err := IsDirectoryEmpty(ctx, ref.ID)
		if err != nil {
			logging.Logger.Error("delete_object_error", zap.Error(err))
			return err
		}
		if !isEmpty {
			return common.NewError("invalid_operation", "Directory is not empty")
		}
		_type = ref.Type
	}
	err := db.Exec("UPDATE reference_objects SET deleted_at=? WHERE lookup_hash=?", allocationVersion, sql.NullTime{
		Time:  common.ToTime(ts),
		Valid: true,
	}, lookupHash).Error
	if err != nil {
		logging.Logger.Error("delete_object_error", zap.Error(err))
		return err
	}
	if _type == FILE {
		deletedRef := &Ref{
			LookupHash: lookupHash,
		}
		collector.DeleteLookupRefRecord(deletedRef)
	}
	return err
}
