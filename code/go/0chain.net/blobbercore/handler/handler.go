// +build !integration_tests

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime/pprof"

	"0chain.net/blobbercore/reference"

	"0chain.net/blobbercore/blobbergrpc"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/constants"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/stats"
	"0chain.net/core/common"

	. "0chain.net/core/logging"
	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

var storageHandler StorageHandler

func GetMetaDataStore() *datastore.Store {
	return datastore.GetStore()
}

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	svc := newGRPCBlobberService(&storageHandler, &packageHandler{})

	//object operations
	r.HandleFunc("/v1/file/upload/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UploadHandler))))
	r.HandleFunc("/v1/file/download/{allocation}", common.UserRateLimit(common.ToByteStream(WithConnection(DownloadHandler))))
	r.HandleFunc("/v1/file/rename/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(RenameHandler))))
	r.HandleFunc("/v1/file/copy/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CopyHandler))))
	r.HandleFunc("/v1/file/attributes/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UpdateAttributesHandler))))

	r.HandleFunc("/v1/connection/commit/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitHandler))))
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitMetaTxnHandler))))
	r.HandleFunc("/v1/file/collaborator/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CollaboratorHandler))))
	r.HandleFunc("/v1/file/calculatehash/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CalculateHashHandler))))

	//object info related apis
	r.HandleFunc("/allocation", common.UserRateLimit(common.ToJSONResponse(WithConnection(AllocationHandler(svc))))).Methods("GET")
	r.HandleFunc("/v1/file/meta/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler(svc))))).Methods("POST")
	r.HandleFunc("/v1/file/stats/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler(svc))))).Methods("POST")
	r.HandleFunc("/v1/file/list/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ListHandler(svc))))).Methods("GET")
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler(svc))))).Methods("GET")
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler(svc))))).Methods("GET")
	r.HandleFunc("/v1/file/objecttree/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectTreeHandler(svc))))).Methods("GET")

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
	return func(ctx context.Context, r *http.Request) (
		resp interface{}, err error) {

		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, err = handler(ctx, r)

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
			return resp, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}
		return
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
	// signature is not requered for all requests, but if header is empty it won`t affect anything
	ctx = context.WithValue(ctx, constants.CLIENT_SIGNATURE_HEADER_KEY, r.Header.Get(common.ClientSignatureHeader))
	return ctx
}

func setupHandlerGRPCContext(r *http.Request) *blobbergrpc.RequestContext {
	var vars = mux.Vars(r)
	return &blobbergrpc.RequestContext{
		Client:          r.Header.Get(common.ClientHeader),
		ClientKey:       r.Header.Get(common.ClientKeyHeader),
		Allocation:      vars["allocation"],
		ClientSignature: r.Header.Get(common.ClientSignatureHeader),
	}
}

func AllocationHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getAllocationResp, err := svc.GetAllocation(ctx, &blobbergrpc.GetAllocationRequest{
			Context: reqCtx,
			Id:      r.FormValue("id"),
		})
		if err != nil {
			return nil, err
		}

		return GRPCAllocationToAllocation(getAllocationResp.Allocation), nil
	}
}

func FileMetaHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getFileMetaDataResp, err := svc.GetFileMetaData(ctx, &blobbergrpc.GetFileMetaDataRequest{
			Context:    reqCtx,
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			AuthToken:  r.FormValue("auth_token"),
			Allocation: reqCtx.Allocation,
		})
		if err != nil {
			return nil, err
		}

		var collaborators []reference.Collaborator
		for _, c := range getFileMetaDataResp.Collaborators {
			collaborators = append(collaborators, GRPCCollaboratorToCollaborator(c))
		}

		result := reference.FileRefGRPCToFileRef(getFileMetaDataResp.MetaData).GetListingData(ctx)
		result["collaborators"] = collaborators

		return result, nil
	}
}

func CommitMetaTxnHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.AddCommitMetaTxn(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func CollaboratorHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.AddCollaborator(ctx, r)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func FileStatsHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getFileStatsResponse, err := svc.GetFileStats(ctx, &blobbergrpc.GetFileStatsRequest{
			Context:    reqCtx,
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			Allocation: reqCtx.Allocation,
		})
		if err != nil {
			return nil, err
		}

		result := reference.FileRefGRPCToFileRef(getFileStatsResponse.MetaData).GetListingData(ctx)

		statsMap := make(map[string]interface{})
		statsBytes, _ := json.Marshal(FileStatsGRPCToFileStats(getFileStatsResponse.Stats))
		if err = json.Unmarshal(statsBytes, &statsMap); err != nil {
			return nil, err
		}
		for k, v := range statsMap {
			result[k] = v
		}

		return result, nil
	}
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
func ListHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		listEntitiesResponse, err := svc.ListEntities(ctx, &blobbergrpc.ListEntitiesRequest{
			Context:    reqCtx,
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			AuthToken:  r.FormValue("auth_token"),
			Allocation: reqCtx.Allocation,
		})
		if err != nil {
			return nil, err
		}

		var entities []map[string]interface{}
		for i := range listEntitiesResponse.Entities {
			entities = append(entities, reference.FileRefGRPCToFileRef(listEntitiesResponse.Entities[i]).GetListingData(ctx))
		}

		return &ListResult{
			AllocationRoot: listEntitiesResponse.AllocationRoot,
			Meta:           reference.FileRefGRPCToFileRef(listEntitiesResponse.MetaData).GetListingData(ctx),
			Entities:       entities,
		}, nil
	}
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

func ReferencePathHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getReferencePathResponse, err := svc.GetReferencePath(ctx, &blobbergrpc.GetReferencePathRequest{
			Context:    reqCtx,
			Paths:      r.FormValue("paths"),
			Path:       r.FormValue("path"),
			Allocation: reqCtx.Allocation,
		})
		if err != nil {
			return nil, err
		}

		var recursionCount int
		return &ReferencePathResult{
			ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getReferencePathResponse.ReferencePath),
			LatestWM:      WriteMarkerGRPCToWriteMarker(getReferencePathResponse.LatestWM),
		}, nil
	}
}

func ObjectPathHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getObjectPathResponse, err := svc.GetObjectPath(ctx, &blobbergrpc.GetObjectPathRequest{
			Context:    reqCtx,
			Allocation: reqCtx.Allocation,
			Path:       r.FormValue("path"),
			BlockNum:   r.FormValue("block_num"),
		})
		if err != nil {
			return nil, err
		}

		path := reference.FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Path).GetListingData(ctx)
		var pathList []map[string]interface{}
		for _, pl := range getObjectPathResponse.ObjectPath.PathList {
			pathList = append(pathList, reference.FileRefGRPCToFileRef(pl).GetListingData(ctx))
		}
		path["list"] = pathList

		return &ObjectPathResult{
			ObjectPath: &reference.ObjectPath{
				RootHash:     getObjectPathResponse.ObjectPath.RootHash,
				Meta:         reference.FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Meta).GetListingData(ctx),
				Path:         path,
				FileBlockNum: getObjectPathResponse.ObjectPath.FileBlockNum,
			},
			LatestWM: WriteMarkerGRPCToWriteMarker(getObjectPathResponse.LatestWriteMarker),
		}, nil
	}
}

func ObjectTreeHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		reqCtx := setupHandlerGRPCContext(r)

		getObjectTreeResponse, err := svc.GetObjectTree(ctx, &blobbergrpc.GetObjectTreeRequest{
			Context:    reqCtx,
			Path:       r.FormValue("path"),
			Allocation: reqCtx.Allocation,
		})
		if err != nil {
			return nil, err
		}

		var recursionCount int
		return &ReferencePathResult{
			ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getObjectTreeResponse.ReferencePath),
			LatestWM:      WriteMarkerGRPCToWriteMarker(getObjectTreeResponse.LatestWM),
		}, nil
	}
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

func UpdateAttributesHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.UpdateObjectAttributes(ctx, r)
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

//nolint:gosimple // need more time to verify
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
