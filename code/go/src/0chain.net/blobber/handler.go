package blobber

import (
	"context"
	"os"
	"runtime/pprof"

	"net/http"

	"0chain.net/common"
	"github.com/gorilla/mux"
)

var storageHandler StorageHandler

const ALLOCATION_CONTEXT_KEY common.ContextKey = "allocation"
const CLIENT_CONTEXT_KEY common.ContextKey = "client"
const CLIENT_KEY_CONTEXT_KEY common.ContextKey = "client_key"

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/file/upload/{allocation}", common.ToJSONResponse(WithConnection(UploadHandler)))
	r.HandleFunc("/v1/file/download/{allocation}", common.ToJSONResponse(WithConnection(DownloadHandler)))
	r.HandleFunc("/v1/file/meta/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(MetaHandler)))
	r.HandleFunc("/v1/file/list/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler)))

	r.HandleFunc("/v1/connection/commit/{allocation}", common.ToJSONResponse(WithConnection(CommitHandler)))
	r.HandleFunc("/v1/connection/details/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(GetConnectionDetailsHandler)))

	r.HandleFunc("/v1/readmarker/latest", common.ToJSONResponse(WithReadOnlyConnection(LatestRMHandler)))
	//r.HandleFunc("/metastore", common.ToJSONResponse(WithConnection(MetaStoreHandler)))
	r.HandleFunc("/debug", common.ToJSONResponse(DumpGoRoutines))

	storageHandler = GetStorageHandler()
}

func WithReadOnlyConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().WithReadOnlyConnection(ctx)
		defer GetMetaDataStore().Discard(ctx)
		res, err := handler(ctx, r)
		return res, err
	}
}

func WithConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().WithConnection(ctx)
		defer GetMetaDataStore().Discard(ctx)
		res, err := handler(ctx, r)
		if err != nil {
			return res, err
		}
		err = GetMetaDataStore().Commit(ctx)
		if err != nil {
			return res, common.NewError("commit_error", "Error committing to meta store")
		}
		return res, err
	}
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}

func LatestRMHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.GetLatestReadMarker(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// func MetaStoreHandler(ctx context.Context, r *http.Request) (interface{}, error) {
// 	operation := r.FormValue("operation")
// 	if operation == "delete" {
// 		err := GetMetaDataStore().DeleteKey(ctx, r.FormValue("key"))
// 		if err != nil {
// 			return nil, err
// 		}
// 		response := make(map[string]string)
// 		response["success"] = "true"
// 		return response, nil
// 	} else if operation == "get" {
// 		dataBytes, err := GetMetaDataStore().ReadBytes(ctx, r.FormValue("key"))
// 		if err != nil {
// 			return nil, err
// 		}
// 		return string(dataBytes), err
// 	}
// 	return nil, common.NewError("invalid_parameters", "Invalid Parameters")
// }

func MetaHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.GetFileMeta(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.WriteFile(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.DownloadFile(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*ListHandler is the handler to respond to upload requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])

	response, err := storageHandler.ListEntities(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func ObjectPathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])

	response, err := storageHandler.GetObjectPathFromBlockNum(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*GetConnectionDetailsHandler is the handler to respond to upload requests fro clients*/
func GetConnectionDetailsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.GetConnectionDetails(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

/*CommitHandler is the handler to respond to upload requests fro clients*/
func CommitHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.CommitWrite(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}
