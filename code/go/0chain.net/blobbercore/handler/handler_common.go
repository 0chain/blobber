package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/build"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
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
		if errors.Is(common.ErrNotFound, err) {
			return response, http.StatusNotFound, nil
		}
		return nil, http.StatusBadRequest, err
	}

	return response, http.StatusOK, nil
}

/*CommitHandler is the handler to respond to upload requests fro clients*/
func commitHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.CommitWrite(ctx, r)
	if err != nil {
		if errors.Is(common.ErrFileWasDeleted, err) {
			return response, http.StatusNoContent, nil
		}
		Logger.Error("Error in handling the request." + err.Error())
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

func WithStatusConnection(handler common.StatusCodeResponderF) common.StatusCodeResponderF {
	return func(ctx context.Context, r *http.Request) (resp interface{}, statusCode int, err error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, statusCode, err = handler(ctx, r)

		defer func() {
			if err != nil {
				var rollErr = GetMetaDataStore().GetTransaction(ctx).
					Rollback().Error
				if rollErr != nil {
					Logger.Error("couldn't rollback", zap.Error(err))
				}
			}
		}()

		if err != nil {
			Logger.Error("Error in handling the request." + err.Error())
			return
		}
		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error
		if err != nil {
			return resp, statusCode, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}
		return
	}
}

func WithStatusReadOnlyConnection(handler common.StatusCodeResponderF) common.StatusCodeResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, int, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, statusCode, err := handler(ctx, r)
		defer func() {
			GetMetaDataStore().GetTransaction(ctx).Rollback()
		}()
		return resp, statusCode, err
	}
}
