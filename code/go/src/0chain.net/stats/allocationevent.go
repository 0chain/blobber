package stats

import (
	"0chain.net/common"
	. "0chain.net/logging"
)

type AllocationEvent struct {
	AllocationID string
}

func (a *AllocationEvent) PerformWork() error {
	if len(a.AllocationID) > 0 {
		ctx := common.GetRootContext()

		bs, bsMutex := NewSyncBlobberStats()
		defer bsMutex.Unlock()

		nctx := GetStatsStore().WithConnection(ctx)
		defer GetStatsStore().Discard(nctx)

		err := bs.NewAllocation(nctx, a.AllocationID)
		if err != nil {
			return err
		}

		err = GetStatsStore().Commit(nctx)
		if err != nil {
			Logger.Error("Error committing the new allocation stats")
		}
		return err
	}
	return common.NewError("invalid_paramaters", "Invalid parameters for allocation stats")
}
