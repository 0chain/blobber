package blobber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime/pprof"

	"0chain.net/allocation"
	"0chain.net/challenge"
	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/stats"
	"0chain.net/writemarker"

	"github.com/gorilla/mux"
)

var storageHandler StorageHandler

const ALLOCATION_CONTEXT_KEY common.ContextKey = "allocation"
const CLIENT_CONTEXT_KEY common.ContextKey = "client"
const CLIENT_KEY_CONTEXT_KEY common.ContextKey = "client_key"

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/file/upload/{allocation}", common.ToJSONResponse(WithConnection(UploadHandler)))
	r.HandleFunc("/v1/file/download/{allocation}", common.ToJSONResponse(WithDownloadStats(WithConnection(DownloadHandler))))
	r.HandleFunc("/v1/file/meta/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler)))
	r.HandleFunc("/v1/file/stats/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler)))
	r.HandleFunc("/v1/file/list/{allocation}", common.ToJSONResponse(WithConnection(ListHandler)))
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler)))
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.ToJSONResponse(WithConnection(ReferencePathHandler)))

	r.HandleFunc("/v1/connection/commit/{allocation}", common.ToJSONResponse(WithUpdateStats(WithConnection(CommitHandler))))
	r.HandleFunc("/v1/connection/details/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(GetConnectionDetailsHandler)))

	r.HandleFunc("/v1/readmarker/latest", common.ToJSONResponse(WithReadOnlyConnection(LatestRMHandler)))
	r.HandleFunc("/v1/challenge/new", common.ToJSONResponse(WithConnection(NewChallengeHandler)))

	r.HandleFunc("/_metastore", common.ToJSONResponse(WithReadOnlyConnection(MetaStoreHandler)))
	r.HandleFunc("/_debug", common.ToJSONResponse(DumpGoRoutines))
	r.HandleFunc("/_config", common.ToJSONResponse(GetConfig))
	r.HandleFunc("/_stats", stats.StatsHandler)
	r.HandleFunc("/_retakechallenge", common.ToJSONResponse(RetakeChallenge))

	storageHandler = GetStorageHandler()
}

//stats.FileBlockDownloaded(ctx, fileref.AllocationID, fileref.Path)

func WithDownloadStats(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		res, err := handler(ctx, r)
		if err != nil {
			return res, err
		}
		response := res.(*DownloadResponse)
		go stats.AddBlockDownloadedStatsEvent(response.AllocationID, response.Path)
		return res, nil
	}
}

func WithUpdateStats(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		res, err := handler(ctx, r)
		if err != nil {
			return res, err
		}
		response := res.(*CommitResult)
		for _, change := range response.Changes {
			if change.Operation == allocation.INSERT_OPERATION || change.Operation == allocation.UPDATE_OPERATION {
				wm := writemarker.Provider().(*writemarker.WriteMarkerEntity)
				wm.WM = response.WriteMarker
				go stats.AddFileUploadedStatsEvent(response.WriteMarker.AllocationID, change.Path, wm.GetKey(), change.Size)
			} else if change.Operation == allocation.DELETE_OPERATION {
				wm := writemarker.Provider().(*writemarker.WriteMarkerEntity)
				wm.WM = response.WriteMarker
				go stats.AddFileDeletedStatsEvent(response.WriteMarker.AllocationID, change.Path, wm.GetKey(), change.Size)
			}
		}

		return res, nil
	}
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

func GetConfig(ctx context.Context, r *http.Request) (interface{}, error) {
	return config.Configuration, nil
}

func NewChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.AcceptChallenge(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
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

func MetaStoreHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	operation := r.FormValue("operation")

	if len(operation) == 0 || operation == "get" {
		key := r.FormValue("key")
		if len(key) > 0 {
			dataBytes, err := GetMetaDataStore().ReadBytes(ctx, key)
			if err != nil {
				return nil, err
			}
			return string(dataBytes), err
		}
		prefix := r.FormValue("prefix")
		if len(prefix) > 0 {
			retObj := make(map[string]interface{})
			iterHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
				jsonObj := make(map[string]interface{})
				bytesReader := bytes.NewBuffer(value)
				d := json.NewDecoder(bytesReader)
				d.UseNumber()
				err := d.Decode(&jsonObj)
				if err != nil {
					return err
				}
				retObj[key] = jsonObj
				return nil
			}
			err := GetMetaDataStore().IteratePrefix(ctx, prefix, iterHandler)
			if err != nil {
				return nil, err
			}
			return retObj, nil
		}

	}
	if operation == "delete" {
		key := r.FormValue("key")
		err := GetMetaDataStore().DeleteKey(ctx, key)
		if err != nil {
			return nil, err
		}
		return "Key deleted", err
	}
	return nil, common.NewError("invalid_parameters", "Invalid Parameters")
}

func FileStatsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))

	response, err := storageHandler.GetFileStats(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func FileMetaHandler(ctx context.Context, r *http.Request) (interface{}, error) {
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
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])

	response, err := storageHandler.ListEntities(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func ReferencePathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, ALLOCATION_CONTEXT_KEY, vars["allocation"])

	response, err := storageHandler.GetReferencePath(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func ObjectPathHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))
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

func RetakeChallenge(ctx context.Context, r *http.Request) (interface{}, error) {
	challengeID := r.FormValue("challenge_id")
	if len(challengeID) == 0 {
		return nil, common.NewError("invalid_parameters", "Please give a valid challenge ID")
	}
	err := challenge.RetakeChallenge(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	return "Challenge triggered again", nil
}
