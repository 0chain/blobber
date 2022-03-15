//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
)

var WriteMarkerMutext = &writemarker.Mutex{}

// LockWriteMarker try to lock writemarker for specified allocation id, and return latest RefTree
func LockWriteMarker(ctx *Context) (interface{}, error) {
	fmt.Println("LockWriteMarker Start !!!")
	defer func() {
		fmt.Println("LockWriteMarker End !!!")
	}()
	connectionID := ctx.FormValue("connection_id")
	requestTime := ctx.FormTime("request_time")

	result, err := WriteMarkerMutext.Lock(ctx, ctx.AllocationTx, connectionID, requestTime)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UnlockWriteMarker release WriteMarkerMutex
func UnlockWriteMarker(ctx *Context) (interface{}, error) {
	fmt.Println("UnlockWriteMarker Start !!!")
	defer func() {
		fmt.Println("UnlockWriteMarker End !!!")
	}()
	connectionID := ctx.Vars["connection"]

	err := WriteMarkerMutext.Unlock(ctx, ctx.AllocationTx, connectionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
