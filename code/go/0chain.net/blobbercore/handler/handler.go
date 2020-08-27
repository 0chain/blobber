// +build !integration_tests

package handler

import (
	"context"
	"net/http"
	"os"
	"runtime/pprof"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/constants"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/stats"
	"0chain.net/core/common"
	. "0chain.net/core/logging"

	"github.com/gorilla/mux"
)

var storageHandler StorageHandler

func GetMetaDataStore() *datastore.Store {
	return datastore.GetStore()
}

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	//object operations
	r.HandleFunc("/v1/file/upload/{allocation}", common.ToJSONResponse(WithConnection(UploadHandler)))
	r.HandleFunc("/v1/file/download/{allocation}", common.ToByteStream(WithConnection(DownloadHandler)))
	r.HandleFunc("/v1/file/rename/{allocation}", common.ToJSONResponse(WithConnection(RenameHandler)))
	r.HandleFunc("/v1/file/copy/{allocation}", common.ToJSONResponse(WithConnection(CopyHandler)))

	r.HandleFunc("/v1/connection/commit/{allocation}", common.ToJSONResponse(WithConnection(CommitHandler)))
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.ToJSONResponse(WithConnection(CommitMetaTxnHandler)))
	r.HandleFunc("/v1/file/calculatehash/{allocation}", common.ToJSONResponse(WithConnection(CalculateHashHandler)))

	//object info related apis
	r.HandleFunc("/allocation", common.ToJSONResponse(WithConnection(AllocationHandler)))
	r.HandleFunc("/v1/file/meta/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler)))
	r.HandleFunc("/v1/file/stats/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler)))
	r.HandleFunc("/v1/file/list/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler)))
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler)))
	r.HandleFunc("/v1/file/objecttree/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ObjectTreeHandler)))

	//admin related
	r.HandleFunc("/_debug", common.ToJSONResponse(DumpGoRoutines))
	r.HandleFunc("/_config", common.ToJSONResponse(GetConfig))
	r.HandleFunc("/_stats", stats.StatsHandler)
	r.HandleFunc("/_statsJSON", common.ToJSONResponse(stats.StatsJSONHandler))
	r.HandleFunc("/_cleanupdisk", common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler)))
	r.HandleFunc("/getstats", common.ToJSONResponse(stats.GetStatsHandler))
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
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY,
		r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY,
		r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY,
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

func FileMetaHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetFileMeta(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func CommitMetaTxnHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.AddCommitMetaTxn(ctx, r)
	if err != nil {
		return nil, err
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

	return response, nil
}

/*ListHandler is the handler to respond to upload requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.ListEntities(ctx, r)
	if err != nil {
		return nil, err
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

	return response, nil
}

func ReferencePathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetReferencePath(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func ObjectPathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetObjectPath(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func ObjectTreeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetObjectTree(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func RenameHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.RenameObject(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func CopyHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.CopyObject(ctx, r)
	if err != nil {
		return nil, err
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

	return response, nil
}

func CalculateHashHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.CalculateHash(ctx, r)
	if err != nil {
		return nil, err
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
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}

func GetConfig(ctx context.Context, r *http.Request) (interface{}, error) {
	return config.Configuration, nil
}

func CleanupDiskHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	err := CleanupDiskFiles(ctx)
	return "cleanup", err
}
