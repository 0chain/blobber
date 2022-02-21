//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
)

var WriteMarkerMutext = &writemarker.Mutex{}

// LockWriteMarker try to lock writemarker for specified allocation id, and return latest RefTree
func LockWriteMarker(ctx *Context) (interface{}, error) {
	sessionID := ctx.FormValue("session_id")
	requestTime := ctx.FormTime("request_time")

	result, err := WriteMarkerMutext.Lock(ctx, ctx.AllocationTx, sessionID, *requestTime)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UnlockWriteMarker release WriteMarkerMutex
func UnlockWriteMarker(ctx *Context) (interface{}, error) {
	sessionID := ctx.FormValue("session_id")

	err := WriteMarkerMutext.Unlock(ctx, ctx.AllocationTx, sessionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
