package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

var WriteMarkerMutext = &writemarker.Mutex{
	ML: common.GetNewLocker(),
}

// LockWriteMarker try to lock writemarker for specified allocation id, and return latest RefTree
func LockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID, _ := ctx.FormValue("connection_id")

	logging.Logger.Info("jayash Allocation Lock : ",
		zap.Any("allocation ID ", ctx.AllocationId),
		zap.Any("ctx", ctx),
		zap.Any("Alloc INput ", ctx.Value("Allocation-Id")),
		zap.Any("connection ID ", connectionID))

	result, err := WriteMarkerMutext.Lock(ctx, ctx.AllocationId, connectionID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UnlockWriteMarker release WriteMarkerMutex
func UnlockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID := ctx.Vars["connection"]

	err := WriteMarkerMutext.Unlock(ctx, ctx.AllocationId, connectionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
