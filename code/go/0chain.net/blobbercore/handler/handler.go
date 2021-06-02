// +build !integration_tests

package handler

import (
	"context"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"

	"google.golang.org/grpc/metadata"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
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

	r.HandleFunc("/v1/connection/commit/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitHandler(svc))))).Methods("POST")
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitMetaTxnHandler))))
	r.HandleFunc("/v1/file/collaborator/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CollaboratorHandler(svc)))))
	r.HandleFunc("/v1/file/calculatehash/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CalculateHashHandler(svc))))).Methods(http.MethodPost)

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

func setupHandlerGRPCContext(ctx context.Context, r *http.Request) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.New(map[string]string{
		common.ClientHeader:          r.Header.Get(common.ClientHeader),
		common.ClientKeyHeader:       r.Header.Get(common.ClientKeyHeader),
		common.ClientSignatureHeader: r.Header.Get(common.ClientSignatureHeader),
	}))
}

func AllocationHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		getAllocationResp, err := svc.GetAllocation(ctx, &blobbergrpc.GetAllocationRequest{
			Id: r.FormValue("id"),
		})
		if err != nil {
			return nil, err
		}

		return convert.GetAllocationResponseHandler(getAllocationResp), nil
	}
}

func FileMetaHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		getFileMetaDataResp, err := svc.GetFileMetaData(ctx, &blobbergrpc.GetFileMetaDataRequest{
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			AuthToken:  r.FormValue("auth_token"),
			Allocation: mux.Vars(r)["allocation"],
		})
		if err != nil {
			return nil, err
		}

		return convert.GetFileMetaDataResponseHandler(getFileMetaDataResp), nil
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

func CollaboratorHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		response, err := svc.Collaborator(ctx, &blobbergrpc.CollaboratorRequest{
			Allocation: mux.Vars(r)["allocation"],
			CollabId:   r.FormValue("collab_id"),
			Method:     r.Method,
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
		})
		if err != nil {
			return nil, err
		}

		return convert.CollaboratorResponse(response), nil
	}
}

func FileStatsHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		getFileStatsResponse, err := svc.GetFileStats(ctx, &blobbergrpc.GetFileStatsRequest{
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			Allocation: mux.Vars(r)["allocation"],
		})
		if err != nil {
			return nil, err
		}

		return convert.GetFileStatsResponseHandler(getFileStatsResponse), nil
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
		ctx = setupHandlerGRPCContext(ctx, r)
		listEntitiesResponse, err := svc.ListEntities(ctx, &blobbergrpc.ListEntitiesRequest{
			Path:       r.FormValue("path"),
			PathHash:   r.FormValue("path_hash"),
			AuthToken:  r.FormValue("auth_token"),
			Allocation: mux.Vars(r)["allocation"],
		})
		if err != nil {
			return nil, err
		}

		return convert.ListEntitesResponseHandler(listEntitiesResponse), nil
	}
}

/*CommitHandler is the handler to respond to upload requests fro clients*/
func CommitHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		response, err := svc.Commit(ctx, &blobbergrpc.CommitRequest{
			Allocation:   mux.Vars(r)["allocation"],
			ConnectionId: r.FormValue("connection_id"),
			WriteMarker:  r.FormValue("write_marker"),
		})
		if err != nil {
			return nil, err
		}

		return convert.CommitWriteResponseHandler(response), nil
	}
}

func ReferencePathHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)
		getReferencePathResponse, err := svc.GetReferencePath(ctx, &blobbergrpc.GetReferencePathRequest{
			Paths:      r.FormValue("paths"),
			Path:       r.FormValue("path"),
			Allocation: mux.Vars(r)["allocation"],
		})
		if err != nil {
			return nil, err
		}

		return convert.GetReferencePathResponseHandler(getReferencePathResponse), nil
	}
}

func ObjectPathHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)
		getObjectPathResponse, err := svc.GetObjectPath(ctx, &blobbergrpc.GetObjectPathRequest{
			Allocation: mux.Vars(r)["allocation"],
			Path:       r.FormValue("path"),
			BlockNum:   r.FormValue("block_num"),
		})
		if err != nil {
			return nil, err
		}

		return convert.GetObjectPathResponseHandler(getObjectPathResponse), nil
	}
}

func ObjectTreeHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)
		getObjectTreeResponse, err := svc.GetObjectTree(ctx, &blobbergrpc.GetObjectTreeRequest{
			Path:       r.FormValue("path"),
			Allocation: mux.Vars(r)["allocation"],
		})
		if err != nil {
			return nil, err
		}

		return convert.GetObjectTreeResponseHandler(getObjectTreeResponse), nil
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

func CalculateHashHandler(svc *blobberGRPCService) func(ctx context.Context, r *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = setupHandlerGRPCContext(ctx, r)

		response, err := svc.CalculateHash(ctx, &blobbergrpc.CalculateHashRequest{
			Allocation: mux.Vars(r)["allocation"],
			Paths:      r.FormValue("paths"),
			Path:       r.FormValue("path"),
		})
		if err != nil {
			return nil, err
		}

		return convert.GetCalculateHashResponseHandler(response), nil
	}
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
