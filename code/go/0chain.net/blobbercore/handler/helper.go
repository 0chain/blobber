package handler

import (
	"context"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/blobbergrpc"
	"0chain.net/blobbercore/constants"
)

func setupGRPCHandlerContext(ctx context.Context, r *blobbergrpc.RequestContext) context.Context {
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY,
		r.Client)
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY,
		r.ClientKey)
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY,
		r.Allocation)
	return ctx
}

func convertAllocationToGRPCAllocation(alloc *allocation.Allocation) *blobbergrpc.Allocation {
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
