package stats

import (
	"0chain.net/common"
	. "0chain.net/logging"
)

const (
	INSERT_UPDATE_OPERATION = 0
	DELETE_OPERATION        = 1
)

type FileUploadedEvent struct {
	AllocationID   string
	Path           string
	WriteMarkerKey string
	Size           int64
	Operation      int
}

func (f *FileUploadedEvent) PerformWork() error {
	if len(f.AllocationID) > 0 && len(f.Path) > 0 {
		ctx := common.GetRootContext()
		fs, fsMutex := NewSyncFileStats(f.AllocationID, f.Path)
		as, asMutex := NewSyncAllocationStats(f.AllocationID)
		bs, bsMutex := NewSyncBlobberStats()
		defer bsMutex.Unlock()
		defer asMutex.Unlock()
		defer fsMutex.Unlock()

		nctx := GetStatsStore().WithConnection(ctx)
		defer GetStatsStore().Discard(nctx)
		if f.Operation == INSERT_UPDATE_OPERATION {
			err := fs.NewWrite(nctx, f)
			if err != nil {
				return err
			}
		} else if f.Operation == DELETE_OPERATION {
			err := GetStatsStore().DeleteKey(nctx, fs.GetKey())
			if err != nil {
				return err
			}
		}

		err := as.NewWrite(nctx, f)
		if err != nil {
			return err
		}
		err = bs.NewWrite(nctx, f)
		if err != nil {
			return err
		}

		err = GetStatsStore().Commit(nctx)
		if err != nil {
			Logger.Error("Error committing the allocation/blobber upload stats")
		}
		return err
	}
	return common.NewError("invalid_paramenters", "Invalid parameters for file updaload stats")
}
