// +build integration_tests

//
// the bad blobber behavior
//

//
package handler

import (
	"context"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/constants"

	"github.com/gorilla/mux"

	// integration tests RPC control
	crpc "github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
)

var storageHandler StorageHandler

func GetMetaDataStore() *datastore.Store {
	return datastore.GetStore()
}

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	//object operations
	r.HandleFunc("/v1/file/upload/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UploadHandler))))
	r.HandleFunc("/v1/file/download/{allocation}", common.UserRateLimit(common.ToByteStream(WithConnection(DownloadHandler))))
	r.HandleFunc("/v1/file/rename/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(RenameHandler))))
	r.HandleFunc("/v1/file/copy/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CopyHandler))))
	r.HandleFunc("/v1/file/attributes/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UpdateObjectAttributes))))

	r.HandleFunc("/v1/connection/commit/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitHandler))))
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitMetaTxnHandler))))

	//object info related apis
	r.HandleFunc("/allocation", common.UserRateLimit(common.ToJSONResponse(WithConnection(AllocationHandler))))
	r.HandleFunc("/v1/file/meta/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler))))
	r.HandleFunc("/v1/file/stats/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler))))
	r.HandleFunc("/v1/file/list/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ListHandler))))
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler))))
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler))))
	r.HandleFunc("/v1/file/objecttree/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectTreeHandler))))

	//admin related
	r.HandleFunc("/_debug", common.UserRateLimit(common.ToJSONResponse(DumpGoRoutines)))
	r.HandleFunc("/_config", common.UserRateLimit(common.ToJSONResponse(GetConfig)))
	r.HandleFunc("/_stats", common.UserRateLimit(stats.StatsHandler))
	r.HandleFunc("/_statsJSON", common.UserRateLimit(common.ToJSONResponse(stats.StatsJSONHandler)))
	r.HandleFunc("/_cleanupdisk", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler))))
	r.HandleFunc("/getstats", common.UserRateLimit(common.ToJSONResponse(stats.GetStatsHandler)))
}

func WithReadOnlyConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		res, err := handler(ctx, r)
		defer func() {
			GetMetaDataStore().GetTransaction(ctx).Rollback()
		}()
		return res, err
	}
}

func WithConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		res, err := handler(ctx, r)
		defer func() {
			if err != nil {
				GetMetaDataStore().GetTransaction(ctx).Rollback()
			}
		}()
		if err != nil {
			Logger.Error("Error in handling the request." + err.Error())
			return res, err
		}
		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error
		if err != nil {
			return res, common.NewError("commit_error", "Error committing to meta store")
		}
		return res, err
	}
}

func setupHandlerContext(ctx context.Context, r *http.Request) context.Context {
	var vars = mux.Vars(r)
	ctx = context.WithValue(ctx, constants.ContextKeyClient,
		r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
		r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyAllocation,
		vars["allocation"])
	return ctx
}

func AllocationHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetAllocationDetails(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func revertString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func FileMetaHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetFileMeta(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		x := response.(map[string]interface{})
		if hash, ok := x["hash"]; ok {
			if str, ok := hash.(string); ok {
				x["hash"] = revertString(str) // provide wrong hash
			}
		}
	}

	return response, nil
}

func CommitMetaTxnHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.AddCommitMetaTxn(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		x := response.(struct {
			Msg string `json:"msg"`
		})
		x.Msg = "Failure" // replace message
		return x, nil
	}

	return response, nil
}

func FileStatsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetFileStats(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.DownloadFile(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		dr := response.(*DownloadResponse)
		dr.Path = "/injection/" + dr.Path
	}

	return response, nil
}

/*ListHandler is the handler to respond to upload requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.ListEntities(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		response.AllocationRoot = revertString(response.AllocationRoot)
	}
	return response, nil
}

/*CommitHandler is the handler to respond to upload requests fro clients*/
func CommitHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.CommitWrite(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		response.AllocationRoot = revertString(response.AllocationRoot)
	}
	return response, nil
}

func ReferencePathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetReferencePath(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		if response.ReferencePath != nil && response.ReferencePath.Meta != nil {
			response.ReferencePath.Meta["hash"] =
				revertString(response.ReferencePath.Meta["hash"].(string))
		}
	}

	return response, nil
}

func ObjectPathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetObjectPath(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		response.FileBlockNum += 20
		response.RootHash = revertString(response.RootHash)
		response.RefID += 12
	}

	return response, nil
}

func ObjectTreeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetObjectTree(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		if len(response.List) > 0 {
			response.List = append(response.List, response.List[0])
		}
	}

	return response, nil
}

func RenameHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.RenameObject(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		ur := response.(*UploadResult)
		ur.Filename = "/injected/" + ur.Filename
	}

	return response, nil
}

func CopyHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.CopyObject(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		ur := response.(*UploadResult)
		ur.Filename = "/injected/" + ur.Filename
	}

	return response, nil
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.WriteFile(ctx, r)
	if err != nil {
		return nil, err
	}

	var state = crpc.Client().State()
	if state.StorageTree.IsBad(state, node.Self.ID) {
		response.Filename = "/injected/" + response.Filename
	}

	return response, nil
}

func HandleShutdown(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			Logger.Info("Shutting down server")
			datastore.GetStore().Close()
		}
	}()
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {
	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}

func GetConfig(ctx context.Context, r *http.Request) (interface{}, error) {
	return config.Configuration, nil
}

func CleanupDiskHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	err := CleanupDiskFiles(ctx)
	return "cleanup", err
}
