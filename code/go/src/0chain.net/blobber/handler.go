package blobber

import (
	"context"

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
	r.HandleFunc("/v1/connection/commit/{allocation}", common.ToJSONResponse(WithConnection(CommitHandler)))
	r.HandleFunc("/v1/connection/details/{allocation}", common.ToJSONResponse(WithConnection(GetConnectionDetailsHandler)))
	// r.HandleFunc("/v1/file/download/{allocation}", DownloadHandler)
	// r.HandleFunc("/v1/file/meta/{allocation}", MetaHandler)
	r.HandleFunc("/v1/file/list/{allocation}", common.ToJSONResponse(WithConnection(ListHandler)))

	// r.HandleFunc("/v1/data/challenge", ChallengeHandler)
	storageHandler = GetStorageHandler()
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
