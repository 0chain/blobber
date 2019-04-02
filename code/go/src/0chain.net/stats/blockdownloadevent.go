package stats

import (
	"0chain.net/common"
	. "0chain.net/logging"
)

type FileDownloadedEvent struct {
	AllocationID string
	Path         string
}

func (f *FileDownloadedEvent) PerformWork() error {
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

		err := fs.NewBlockDownload(nctx, f)
		if err != nil {
			return err
		}
		err = as.NewBlockDownload(nctx, f)
		if err != nil {
			return err
		}
		err = bs.NewBlockDownload(nctx, f)
		if err != nil {
			return err
		}

		err = GetStatsStore().Commit(nctx)
		if err != nil {
			Logger.Error("Error committing the allocation/blobber download stats")
		}
		return err
	}
	return common.NewError("invalid_paramenters", "Invalid parameters for file downloaded stats")

}
