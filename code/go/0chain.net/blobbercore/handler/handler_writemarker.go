package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// LockWriteMarker try to lock writemarker for specified allocation id, and return latest RefTree
func LockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID, _ := ctx.FormValue("connection_id")

	result, err := writemarker.WriteMarkerMutext.Lock(ctx, ctx.AllocationId, connectionID)
	Logger.Info("Lock write marker result", zap.Any("result", result), zap.Error(err))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UnlockWriteMarker release WriteMarkerMutex
func UnlockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID := ctx.Vars["connection"]

	err := writemarker.WriteMarkerMutext.Unlock(ctx.AllocationId, connectionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
