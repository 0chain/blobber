package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// swagger:route POST /v1/writemarker/lock/{allocation} PostLockWriteMarker
// Lock a write marker.
// LockWriteMarker try to lock writemarker for specified allocation id.
//
// parameters:
//   +name: allocation
//     in: path
//     type: string
//     required: true
//     description: allocation id
//	 +name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: ALLOCATION-ID
//	   description: The ID of the allocation in question.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature
//     description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//     in: header
//     type: string
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request if the X-Version is "v2"
//     in: header
//     type: string
//  +name: connection_id
//     description: The ID of the connection associated with the write marker.
//     in: query
//     type: string
//     required: true
//
// responses:
//   200: WriteMarkerLockResult
//   400:
//   500:
func LockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID, _ := ctx.FormValue("connection_id")

	result, err := writemarker.WriteMarkerMutext.Lock(ctx, ctx.AllocationId, connectionID)
	Logger.Info("Lock write marker result", zap.Any("result", result), zap.Error(err))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// swagger:route DELETE /v1/writemarker/lock/{allocation}/{connection} DeleteLockWriteMarker
// Unlock a write marker.
// UnlockWriteMarker release WriteMarkerMutex locked by the Write Marker Lock endpoint.
//
// parameters:
//   +name: allocation
//     in: path
//     type: string
//     required: true
//     description: allocation id
//   +name: connection
//     in: path
//     type: string
//     required: true
//     description: connection id associae with the write marker
//	 +name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: ALLOCATION-ID
//	   description: The ID of the allocation in question.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature
//     description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//     in: header
//     type: string
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request if the X-Version is "v2"
//     in: header
//     type: string
//
// responses:
//   200:
//   400:
//   500:
func UnlockWriteMarker(ctx *Context) (interface{}, error) {
	connectionID := ctx.Vars["connection"]

	err := writemarker.WriteMarkerMutext.Unlock(ctx.AllocationId, connectionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
