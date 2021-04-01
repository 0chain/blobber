package handler

import (
	"context"

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
