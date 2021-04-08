package handler

import (
	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/blobbergrpc"
	"0chain.net/blobbercore/stats"
	"0chain.net/blobbercore/writemarker"
)

func AllocationToGRPCAllocation(alloc *allocation.Allocation) *blobbergrpc.Allocation {
	terms := make([]*blobbergrpc.Term, len(alloc.Terms))
	for _, t := range alloc.Terms {
		terms = append(terms, &blobbergrpc.Term{
			ID:           t.ID,
			BlobberID:    t.BlobberID,
			AllocationID: t.AllocationID,
			ReadPrice:    t.ReadPrice,
			WritePrice:   t.WritePrice,
		})
	}
	return &blobbergrpc.Allocation{
		ID:               alloc.ID,
		Tx:               alloc.Tx,
		TotalSize:        alloc.TotalSize,
		UsedSize:         alloc.UsedSize,
		OwnerID:          alloc.OwnerID,
		OwnerPublicKey:   alloc.OwnerPublicKey,
		Expiration:       int64(alloc.Expiration),
		AllocationRoot:   alloc.AllocationRoot,
		BlobberSize:      alloc.BlobberSize,
		BlobberSizeUsed:  alloc.BlobberSizeUsed,
		LatestRedeemedWM: alloc.LatestRedeemedWM,
		IsRedeemRequired: alloc.IsRedeemRequired,
		TimeUnit:         int64(alloc.TimeUnit),
		CleanedUp:        alloc.CleanedUp,
		Finalized:        alloc.Finalized,
		Terms:            terms,
		PayerID:          alloc.PayerID,
	}
}

func FileStatsToFileStatsGRPC(fileStats *stats.FileStats) *blobbergrpc.FileStats {
	if fileStats == nil {
		return &blobbergrpc.FileStats{}
	}

	return &blobbergrpc.FileStats{
		ID:                       fileStats.ID,
		RefID:                    fileStats.RefID,
		NumUpdates:               fileStats.NumUpdates,
		NumBlockDownloads:        fileStats.NumBlockDownloads,
		SuccessChallenges:        fileStats.SuccessChallenges,
		FailedChallenges:         fileStats.FailedChallenges,
		LastChallengeResponseTxn: fileStats.LastChallengeResponseTxn,
		WriteMarkerRedeemTxn:     fileStats.WriteMarkerRedeemTxn,
		CreatedAt:                fileStats.CreatedAt.UnixNano(),
		UpdatedAt:                fileStats.UpdatedAt.UnixNano(),
	}
}

func WriteMarkerToWriteMarkerGRPC(wm writemarker.WriteMarker) *blobbergrpc.WriteMarker {
	return &blobbergrpc.WriteMarker{
		AllocationRoot:         wm.AllocationRoot,
		PreviousAllocationRoot: wm.PreviousAllocationRoot,
		AllocationID:           wm.AllocationID,
		Size:                   wm.Size,
		BlobberID:              wm.BlobberID,
		Timestamp:              int64(wm.Timestamp),
		ClientID:               wm.ClientID,
		Signature:              wm.Signature,
	}
}
