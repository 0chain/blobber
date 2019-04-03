package stats

import (
	"0chain.net/common"
	. "0chain.net/logging"
	"go.uber.org/zap"
)

type ChallengeStatus int
type ChallengeRedeemStatus int

const (
	NEW     ChallengeStatus = 0
	SUCCESS ChallengeStatus = 1
	FAILED  ChallengeStatus = 2

	NOTREDEEMED   ChallengeRedeemStatus = 0
	REDEEMSUCCESS ChallengeRedeemStatus = 1
	REDEEMERROR   ChallengeRedeemStatus = 2
)

type ChallengeEvent struct {
	AllocationID string
	ChallengeID  string
	Path         string
	Result       ChallengeStatus
	RedeemStatus ChallengeRedeemStatus
	RedeemTxn    string
}

func (f *ChallengeEvent) PerformWork() error {
	if f.Result == NEW && len(f.AllocationID) > 0 {
		ctx := common.GetRootContext()

		as, asMutex := NewSyncAllocationStats(f.AllocationID)
		bs, bsMutex := NewSyncBlobberStats()
		defer bsMutex.Unlock()
		defer asMutex.Unlock()

		nctx := GetStatsStore().WithConnection(ctx)
		defer GetStatsStore().Discard(nctx)

		err := as.NewChallenge(nctx, f)
		if err != nil {
			return err
		}
		err = bs.NewChallenge(nctx, f)
		if err != nil {
			return err
		}

		Logger.Info("New challenge received", zap.Any("as", as), zap.Any("bs", bs))

		err = GetStatsStore().Commit(nctx)
		if err != nil {
			Logger.Error("Error committing the allocation/blobber new challenge stats")
		}
		return err
	} else if len(f.Path) > 0 && len(f.AllocationID) > 0 && f.RedeemStatus != NOTREDEEMED && len(f.RedeemTxn) > 0 {
		ctx := common.GetRootContext()
		fs, fsMutex := NewSyncFileStats(f.AllocationID, f.Path)
		as, asMutex := NewSyncAllocationStats(f.AllocationID)
		bs, bsMutex := NewSyncBlobberStats()
		defer bsMutex.Unlock()
		defer asMutex.Unlock()
		defer fsMutex.Unlock()

		nctx := GetStatsStore().WithConnection(ctx)
		defer GetStatsStore().Discard(nctx)

		err := fs.ChallengeRedeemed(nctx, f)
		if err != nil {
			return err
		}
		err = as.ChallengeRedeemed(nctx, f)
		if err != nil {
			return err
		}
		err = bs.ChallengeRedeemed(nctx, f)
		if err != nil {
			return err
		}

		err = GetStatsStore().Commit(nctx)
		if err != nil {
			Logger.Error("Error committing the allocation/blobber new challenge stats")
		}
		return err
	}

	return common.NewError("invalid_paramenters", "Invalid parameters for file updaload stats")
}
