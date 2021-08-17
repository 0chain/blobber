// +build !integration_tests

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"gorm.io/gorm"

	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/mux"
)

var storageHandler StorageHandler

func GetMetaDataStore() *datastore.Store {
	return datastore.GetStore()
}

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	//object operations
	r.HandleFunc("/v1/file/upload/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UploadHandler))))
	r.HandleFunc("/v1/file/download/{allocation}", common.UserRateLimit(common.ToByteStream(WithConnection(DownloadHandler)))).Methods("POST")
	r.HandleFunc("/v1/file/rename/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(RenameHandler))))
	r.HandleFunc("/v1/file/copy/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CopyHandler))))
	r.HandleFunc("/v1/file/attributes/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(UpdateAttributesHandler))))
	r.HandleFunc("/v1/dir/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CreateDirHandler)))).Methods("POST")
	r.HandleFunc("/v1/dir/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CreateDirHandler)))).Methods("DELETE")
	r.HandleFunc("/v1/dir/rename/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CreateDirHandler)))).Methods("POST")

	r.HandleFunc("/v1/connection/commit/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitHandler))))
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CommitMetaTxnHandler))))
	r.HandleFunc("/v1/file/collaborator/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CollaboratorHandler))))
	r.HandleFunc("/v1/file/calculatehash/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(CalculateHashHandler))))

	//object info related apis
	r.HandleFunc("/allocation", common.UserRateLimit(common.ToJSONResponse(WithConnection(AllocationHandler))))
	r.HandleFunc("/v1/file/meta/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler))))
	r.HandleFunc("/v1/file/stats/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler))))
	r.HandleFunc("/v1/file/list/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ListHandler))))
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler))))
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler))))
	r.HandleFunc("/v1/file/objecttree/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(ObjectTreeHandler))))
	r.HandleFunc("/v1/file/refs/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(RefsHandler)))).Methods("GET")
	//admin related
	r.HandleFunc("/_debug", common.UserRateLimit(common.ToJSONResponse(DumpGoRoutines)))
	r.HandleFunc("/_config", common.UserRateLimit(common.ToJSONResponse(GetConfig)))
	r.HandleFunc("/_stats", common.UserRateLimit(stats.StatsHandler))
	r.HandleFunc("/_statsJSON", common.UserRateLimit(common.ToJSONResponse(stats.StatsJSONHandler)))
	r.HandleFunc("/_cleanupdisk", common.UserRateLimit(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler))))
	r.HandleFunc("/getstats", common.UserRateLimit(common.ToJSONResponse(stats.GetStatsHandler)))

	//marketplace related
	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}", common.UserRateLimit(common.ToJSONResponse(WithConnection(MarketPlaceShareInfoHandler))))
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

func CollaboratorHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.AddCollaborator(ctx, r)
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
	ctx, canceler := context.WithTimeout(ctx, time.Second*10)
	defer canceler()

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

func RefsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetRefs(ctx, r)
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

/*CreateDirHandler is the handler to respond to create dir for allocation*/
func CreateDirHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.CreateDir(ctx, r)
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

func RevokeShare(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)
	allocation, ok := mux.Vars(r)["allocation"]
	if !ok {
		return false, common.NewError("invalid_params", "Missing allocation tx")
	}
	valid, err := verifySignatureFromRequest(allocation, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	path := r.FormValue("path")
	refereeClientID := r.FormValue("refereeClientID")
	filePathHash := fileref.GetReferenceLookup(allocationID, path)
	_, err = reference.GetReferenceFromLookupHash(ctx, allocationID, filePathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	err = reference.DeleteShareInfo(ctx, reference.ShareInfo{
		ClientID:     refereeClientID,
		FilePathHash: filePathHash,
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		resp := map[string]interface{}{
			"status":  http.StatusNotFound,
			"message": "Path not found",
		}
		return resp, nil
	}
	if err != nil {
		return nil, err
	}
	resp := map[string]interface{}{
		"status":  http.StatusNoContent,
		"message": "Path successfully removed from allocation",
	}
	return resp, nil
}

func InsertShare(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)

	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)
	allocation, ok := mux.Vars(r)["allocation"]
	if !ok {
		return false, common.NewError("invalid_params", "Missing allocation tx")
	}
	valid, err := verifySignatureFromRequest(allocation, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	encryptionPublicKey := r.FormValue("encryption_public_key")
	authTicketString := r.FormValue("auth_ticket")
	authTicket := &readmarker.AuthTicket{}

	err = json.Unmarshal([]byte(authTicketString), &authTicket)
	if err != nil {
		return false, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, authTicket.FilePathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	authTicketVerified, err := storageHandler.verifyAuthTicket(ctx, authTicketString, allocationObj, fileref, authTicket.ClientID)
	if !authTicketVerified {
		return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket. "+err.Error())
	}

	if err != nil {
		return nil, err
	}

	shareInfo := reference.ShareInfo{
		OwnerID:                   authTicket.OwnerID,
		ClientID:                  authTicket.ClientID,
		FilePathHash:              authTicket.FilePathHash,
		ReEncryptionKey:           authTicket.ReEncryptionKey,
		ClientEncryptionPublicKey: encryptionPublicKey,
		ExpiryAt:                  common.ToTime(authTicket.Expiration),
	}

	existingShare, _ := reference.GetShareInfo(ctx, authTicket.ClientID, authTicket.FilePathHash)

	if existingShare != nil && len(existingShare.OwnerID) > 0 {
		err = reference.UpdateShareInfo(ctx, shareInfo)
	} else {
		err = reference.AddShareInfo(ctx, shareInfo)
	}
	if err != nil {
		return nil, err
	}

	resp := map[string]interface{}{
		"message": "Share info added successfully",
	}

	return resp, nil
}

func MarketPlaceShareInfoHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "DELETE" {
		return RevokeShare(ctx, r)
	}

	if r.Method == "POST" {
		return InsertShare(ctx, r)
	}

	return nil, errors.New("invalid request method, only POST is allowed")
}
