//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

var storageHandler StorageHandler

func GetMetaDataStore() datastore.Store {
	return datastore.GetStore()
}

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.Use(useRecovery, useCors, common.UseUserRateLimit)

	//object operations
	r.HandleFunc("/v1/file/upload/{allocation}", common.ToJSONResponse(WithConnection(UploadHandler)))
	r.HandleFunc("/v1/file/download/{allocation}", common.ToByteStream(WithConnection(DownloadHandler))).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/v1/file/rename/{allocation}", common.ToJSONResponse(WithConnection(RenameHandler))).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/v1/file/copy/{allocation}", common.ToJSONResponse(WithConnection(CopyHandler))).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/v1/file/attributes/{allocation}", common.ToJSONResponse(WithConnection(UpdateAttributesHandler)))
	r.HandleFunc("/v1/dir/{allocation}", common.ToJSONResponse(WithConnection(CreateDirHandler))).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/v1/dir/{allocation}", common.ToJSONResponse(WithConnection(CreateDirHandler))).Methods(http.MethodDelete, http.MethodOptions)

	r.HandleFunc("/v1/connection/commit/{allocation}", common.ToStatusCode(WithStatusConnection(CommitHandler)))
	r.HandleFunc("/v1/file/commitmetatxn/{allocation}", common.ToJSONResponse(WithConnection(CommitMetaTxnHandler)))
	r.HandleFunc("/v1/file/collaborator/{allocation}", common.ToJSONResponse(WithConnection(CollaboratorHandler)))
	r.HandleFunc("/v1/file/calculatehash/{allocation}", common.ToJSONResponse(WithConnection(CalculateHashHandler)))

	//object info related apis
	r.HandleFunc("/allocation", common.ToJSONResponse(WithConnection(AllocationHandler)))
	r.HandleFunc("/v1/file/meta/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler)))
	r.HandleFunc("/v1/file/stats/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler)))
	r.HandleFunc("/v1/file/list/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ListHandler))).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/v1/file/objectpath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ObjectPathHandler)))
	r.HandleFunc("/v1/file/referencepath/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler)))
	r.HandleFunc("/v1/file/objecttree/{allocation}", common.ToStatusCode(WithStatusReadOnlyConnection(ObjectTreeHandler))).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/v1/file/refs/{allocation}", common.ToJSONResponse(WithReadOnlyConnection(RefsHandler))).Methods(http.MethodGet, http.MethodOptions)
	//admin related
	r.HandleFunc("/_debug", common.ToJSONResponse(DumpGoRoutines))
	r.HandleFunc("/_config", common.ToJSONResponse(GetConfig))
	r.HandleFunc("/_stats", StatsHandler)
	r.HandleFunc("/_statsJSON", common.ToJSONResponse(stats.StatsJSONHandler))
	r.HandleFunc("/_cleanupdisk", common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler)))
	r.HandleFunc("/getstats", common.ToJSONResponse(stats.GetStatsHandler))

	//marketplace related
	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}", common.ToJSONResponse(WithConnection(InsertShare))).Methods(http.MethodOptions, http.MethodPost)
	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}", common.ToJSONResponse(WithConnection(RevokeShare))).Methods(http.MethodOptions, http.MethodDelete)

	// lightweight http handler without heavy postgres transaction to improve performance

	r.HandleFunc("/v1/writemarker/lock/{allocation}", WithHandler(LockWriteMarker)).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/v1/writemarker/lock/{allocation}/{connection}", WithHandler(UnlockWriteMarker)).Methods(http.MethodDelete, http.MethodOptions)

	r.HandleFunc("/v1/hashnode/root/{allocation}", WithHandler(LoadRootHashnode)).Methods(http.MethodGet, http.MethodOptions)
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
	return func(ctx context.Context, r *http.Request) (resp interface{}, err error) {
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
	ctx = context.WithValue(ctx, constants.ContextKeyClient,
		r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
		r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyAllocation,
		vars["allocation"])
	// signature is not requered for all requests, but if header is empty it won`t affect anything
	ctx = context.WithValue(ctx, constants.ContextKeyClientSignatureHeaderKey, r.Header.Get(common.ClientSignatureHeader))
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
	return storageHandler.DownloadFile(ctx, r)
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

/*CommitHandler is the handler to respond to upload requests from clients*/
func CommitHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return commitHandler(ctx, r)
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

func ObjectTreeHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return objectTreeHandler(ctx, r)
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

func writeResponse (w http.ResponseWriter, resp []byte) {
	_, err := w.Write(resp)

	if err != nil {
		Logger.Error("Error sending StatsHandler response", zap.Error(err))
	}
}

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	isJSON := r.Header.Get("Accept") == "application/json"

	if isJSON {
		blobberInfo := GetBlobberInfoJson()

		ctx := datastore.GetStore().CreateTransaction(r.Context())
		blobberStats, err := stats.StatsJSONHandler(ctx, r)

		if err != nil {
			Logger.Error("Error getting blobber JSON stats", zap.Error(err))

			w.WriteHeader(http.StatusInternalServerError)
			writeResponse(w, []byte(err.Error()))
		}

		blobberInfo.Stats = blobberStats

		statsJson, err := json.Marshal(blobberInfo)

		if err != nil {
			Logger.Error("Error marshaling JSON stats", zap.Error(err))

			w.WriteHeader(http.StatusInternalServerError)
			writeResponse(w, []byte(err.Error()))
		}

		writeResponse(w, statsJson)

		return
	}

	HTMLHeader(w, "Blobber Diagnostics")
	PrintCSS(w)
	HomepageHandler(w, r)

	if getBlobberHealthCheckError() != nil {
		r.Header.Set(stats.HealthDataKey.String(), "✗")
	} else {
		r.Header.Set(stats.HealthDataKey.String(), "✔")
	}

	stats.StatsHandler(w, r)
	HTMLFooter(w)
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

	allocationID := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)

	valid, err := verifySignatureFromRequest(allocationID, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	path := r.FormValue("path")
	refereeClientID := r.FormValue("refereeClientID")
	filePathHash := fileref.GetReferenceLookup(allocationID, path)
	//_, err = reference.GetReferenceByLookupHashForAddCollaborator(ctx, allocationID, filePathHash)
	_, err = reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, filePathHash, []string{"id", "type"})
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	err = reference.DeleteShareInfo(ctx, &reference.ShareInfo{
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

	var (
		allocationID = ctx.Value(constants.ContextKeyAllocation).(string)
		clientID     = ctx.Value(constants.ContextKeyClient).(string)
	)

	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)

	valid, err := verifySignatureFromRequest(allocationID, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_client", "Client has no access to share file")
	}

	encryptionPublicKey := r.FormValue("encryption_public_key")
	authTicketString := r.FormValue("auth_ticket")
	availableAfter := r.FormValue("available_after")
	authTicket := &readmarker.AuthTicket{}

	err = json.Unmarshal([]byte(authTicketString), &authTicket)
	if err != nil {
		return false, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
	}
	fileRef, err := reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, authTicket.FilePathHash, []string{"id", "path", "lookup_hash", "type", "name"})
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	authToken, err := storageHandler.verifyAuthTicket(ctx, authTicketString, allocationObj, fileRef, authTicket.ClientID)
	if authToken == nil {
		return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket. "+err.Error())
	}

	if err != nil {
		return nil, err
	}

	availableAt := common.Now()

	if len(availableAfter) > 0 {
		a, err := strconv.ParseInt(availableAfter, 10, 64)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid available_after: "+err.Error())
		}
		availableAt = common.Timestamp(a)
	}

	shareInfo := reference.ShareInfo{
		OwnerID:                   authTicket.OwnerID,
		ClientID:                  authTicket.ClientID,
		FilePathHash:              authTicket.FilePathHash,
		ReEncryptionKey:           authTicket.ReEncryptionKey,
		ClientEncryptionPublicKey: encryptionPublicKey,
		ExpiryAt:                  common.ToTime(authTicket.Expiration),
		AvailableAt:               common.ToTime(availableAt),
	}

	existingShare, _ := reference.GetShareInfo(ctx, authTicket.ClientID, authTicket.FilePathHash)

	if existingShare != nil && len(existingShare.OwnerID) > 0 {
		err = reference.UpdateShareInfo(ctx, &shareInfo)
	} else {
		err = reference.AddShareInfo(ctx, &shareInfo)
	}

	if err != nil {
		Logger.Info(err.Error())
		return nil, common.NewError("share_info_insert", "Unable to save share info")
	}

	return map[string]interface{}{"message": "Share info added successfully"}, nil
}

//PrintCSS - print the common css elements
func PrintCSS(w http.ResponseWriter) {
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, ".number { text-align: right; }\n")
	fmt.Fprintf(w, ".fixed-text { overflow:hidden;white-space: nowrap;word-break: break-all;word-wrap: break-word; text-overflow: ellipsis; }\n")
	fmt.Fprintf(w, ".menu li { list-style-type: none; }\n")
	fmt.Fprintf(w, ".page-item { display:inline-block; padding:10px;}\n")
	fmt.Fprintf(w, ".pagination {float: right; margin-right: 24px;}\n")
	fmt.Fprintf(w, "table, td, th { border: 1px solid black;  border-collapse: collapse;}\n")
	fmt.Fprintf(w, ".tname { width: 70%%}\n")
	fmt.Fprintf(w, "tr.header { background-color: #E0E0E0;  }\n")
	fmt.Fprintf(w, ".inactive { background-color: #F44336; }\n")
	fmt.Fprintf(w, ".warning { background-color: #FFEB3B; }\n")
	fmt.Fprintf(w, ".optimal { color: #1B5E20; }\n")
	fmt.Fprintf(w, ".slow { font-style: italic; }\n")
	fmt.Fprintf(w, ".bold {font-weight:bold;}")
	fmt.Fprintf(w, "tr.green td {background-color:light-green;}")
	fmt.Fprintf(w, "tr.grey td {background-color:light-grey;}")
	fmt.Fprintf(w, "</style>")
}

func HTMLHeader(w http.ResponseWriter, title string) {
	fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>%s</title></head><body>", title)
}
func HTMLFooter(w http.ResponseWriter) {
	fmt.Fprintf(w, "</body></html>")
}
