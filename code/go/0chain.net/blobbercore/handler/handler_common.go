package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/build"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

var StartTime time.Time

func objectTreeHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.GetObjectTree(ctx, r)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return response, http.StatusNotFound, nil
		}
		Logger.Error("objectTreeHandler_request_failed", zap.Error(err))
		return nil, http.StatusBadRequest, err
	}

	return response, http.StatusOK, nil
}

/*CommitHandler is the handler to respond to upload requests from clients*/
func commitHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.CommitWrite(ctx, r)
	if err != nil {
		if errors.Is(err, common.ErrFileWasDeleted) {
			return response, http.StatusNoContent, nil
		}
		Logger.Error("commitHandler_request_failed", zap.Error(err))
		return nil, http.StatusBadRequest, err
	}

	return response, http.StatusOK, nil
}

func commitHandlerV2(ctx context.Context, r *http.Request) (any, int, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.CommitWriteV2(ctx, r)
	if err != nil {
		if errors.Is(err, common.ErrFileWasDeleted) {
			return response, http.StatusNoContent, nil
		}
		Logger.Error("commitHandler_request_failed", zap.Error(err))
		return nil, http.StatusBadRequest, err
	}

	return response, http.StatusOK, nil
}

// RollbackHandler is the handler to respond to upload requests from clients
func rollbackHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.Rollback(ctx, r)
	if err != nil {
		if errors.Is(err, common.ErrFileWasDeleted) {
			return response, http.StatusNoContent, nil
		}
		Logger.Error("rollbackHandler_request_failed", zap.Error(err))
		return nil, http.StatusBadRequest, err
	}

	return response, http.StatusOK, nil
}

func HomepageHandler(w http.ResponseWriter, r *http.Request) {
	mc := chain.GetServerChain()

	fmt.Fprintf(w, "<div>Working on the chain: %v</div>\n", mc.ID)
	fmt.Fprintf(w,
		"<div>I am a blobber with <ul><li>id:%v</li><li>public_key:%v</li><li>build_tag:%v</li></ul></div>\n",
		node.Self.ID, node.Self.PublicKey, build.BuildTag,
	)

	fmt.Fprintf(w, "<div>Miners ...\n")
	network := zcncore.GetNetwork()
	for _, miner := range network.Miners {
		fmt.Fprintf(w, "%v\n", miner)
	}
	fmt.Fprintf(w, "</div>\n")
	fmt.Fprintf(w, "<div>Sharders ...\n")
	for _, sharder := range network.Sharders {
		fmt.Fprintf(w, "%v\n", sharder)
	}
	fmt.Fprintf(w, "</div>\n")
	fmt.Fprintf(w, "</br>")
	fmt.Fprintf(w, "<div>Running since %v (Total elapsed time: %v)</div>\n", StartTime.Format(common.DateTimeFormat), time.Since(StartTime))
	fmt.Fprintf(w, "</br>")
}

type BlobberInfo struct {
	ChainId          string      `json:"chain_id"`
	BlobberId        string      `json:"blobber_id"`
	BlobberPublicKey string      `json:"public_key"`
	BuildTag         string      `json:"build_tag"`
	Stats            interface{} `json:"stats"`
}

func GetBlobberInfoJson() BlobberInfo {
	mc := chain.GetServerChain()

	blobberInfo := BlobberInfo{
		ChainId:          mc.ID,
		BlobberId:        node.Self.ID,
		BlobberPublicKey: node.Self.PublicKey,
		BuildTag:         build.BuildTag,
	}

	return blobberInfo
}

// Should only be used for handlers where the writemarker is submitted
func WithStatusConnectionForWM(handler common.StatusCodeResponderF) common.StatusCodeResponderF {
	return func(ctx context.Context, r *http.Request) (resp interface{}, statusCode int, err error) {
		allocationID := r.Header.Get(common.AllocationIdHeader)
		if allocationID == "" {
			return nil, http.StatusBadRequest, common.NewError("invalid_allocation_id", "Allocation ID is required")
		}

		// Lock will compete with other CommitWrites and Challenge validation

		mutex := lock.GetMutex(allocation.Allocation{}.TableName(), allocationID)
		Logger.Info("Locking allocation", zap.String("allocation_id", allocationID))
		wmSet := writemarker.SetCommittingMarker(allocationID, true)
		if !wmSet {
			return nil, http.StatusBadRequest, common.NewError("pending_markers", "Committing marker set failed")
		}
		mutex.Lock()
		defer mutex.Unlock()
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		tx := GetMetaDataStore().GetTransaction(ctx)
		resp, statusCode, err = handler(ctx, r)

		defer func() {
			if err != nil {
				var rollErr = tx.
					Rollback().Error
				if rollErr != nil {
					Logger.Error("couldn't rollback", zap.Error(err))
				}
				writemarker.SetCommittingMarker(allocationID, false)
			}
		}()

		if err != nil {
			Logger.Error("Error in handling the request." + err.Error())
			return
		}
		err = tx.Commit().Error
		if err != nil {
			if blobberRes, ok := resp.(*blobberhttp.CommitResult); ok {
				trie := blobberRes.Trie
				if trie != nil {
					trie.Rollback()
					blobberRes.Trie = nil
				}
			}
			Logger.Error("Error committing to meta store", zap.Error(err))
			return resp, http.StatusInternalServerError, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}

		Logger.Info("commit_success", zap.String("allocation_id", allocationID), zap.Any("response", resp))

		if blobberRes, ok := resp.(*blobberhttp.CommitResult); ok {
			// Save the write marker data
			writemarker.SaveMarkerData(allocationID, blobberRes.WriteMarker.WM.Timestamp, blobberRes.WriteMarker.WM.ChainLength)
			trie := blobberRes.Trie
			if trie != nil {
				_ = trie.DeleteNodes()
				blobberRes.Trie = nil
			}
		} else {
			Logger.Error("Invalid response type for commit handler")
			return resp, http.StatusInternalServerError, common.NewError("invalid_response_type", "Invalid response type for commit handler")
		}
		return
	}
}

func WithStatusReadOnlyConnection(handler common.StatusCodeResponderF) common.StatusCodeResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, int, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		tx := GetMetaDataStore().GetTransaction(ctx)
		defer func() {
			tx.Rollback()
		}()
		resp, statusCode, err := handler(ctx, r)
		return resp, statusCode, err
	}
}
