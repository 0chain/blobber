package handler

import (
	"strings"
	"0chain.net/blobbercore/stats"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/constants"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/readmarker"
	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/writemarker"
	"0chain.net/core/common"
	"0chain.net/core/node"
	"0chain.net/core/encryption"
	"0chain.net/core/lock"
	. "0chain.net/core/logging"
	
	"github.com/jinzhu/gorm"
	"go.uber.org/zap"
)

const FORM_FILE_PARSE_MAX_MEMORY = 10 * 1024 * 1024
const (
	DOWNLOAD_CONTENT_FULL = "full"
	DOWNLOAD_CONTENT_THUMB = "thumbnail"
)


type StorageHandler struct {
}

func (fsh *StorageHandler) verifyAllocation(ctx context.Context, allocationID string, readonly bool) (*allocation.Allocation, error) {
	if len(allocationID) == 0 {
		return nil, common.NewError("invalid_allocation", "Invalid allocation id")
	}
	allocationObj, err := allocation.VerifyAllocationTransaction(ctx, allocationID, readonly)
	if err != nil {
		return nil, err
	}
	return allocationObj, nil
}

func (fsh *StorageHandler) verifyAuthTicket(ctx context.Context, r *http.Request, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error) {
	authTokenString := r.FormValue("auth_token")
	if len(authTokenString) == 0 {
		return false, common.NewError("invalid_parameters", "Auth ticket required if data read by anyone other than owner.")
	}
	authToken := &readmarker.AuthTicket{}
	err := json.Unmarshal([]byte(authTokenString), &authToken)
	if err != nil {
		return false, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
	}
	err = authToken.Verify(allocationObj, clientID)
	if err != nil {
		return false, err
	}
	if refRequested.LookupHash != authToken.FilePathHash {
		authTokenRef, err := reference.GetReferenceFromLookupHash(ctx, authToken.AllocationID, authToken.FilePathHash)
		if err != nil {
			return false, err
		}
		if refRequested.ParentPath != authTokenRef.Path && !strings.HasPrefix(refRequested.ParentPath, authTokenRef.Path + "/") {
			return false, common.NewError("invalid_parameters", "Auth ticket is not valid for the resource being requested")
		}
	}
	
	
	return true, nil
}

func (fsh *StorageHandler) GetAllocationDetails(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method != "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := r.FormValue("id")
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	return allocationObj, nil
}

func (fsh *StorageHandler) checkIfFileAlreadyExists(ctx context.Context, allocationID string, path string) *reference.Ref {
	fileReference, err := reference.GetReference(ctx, allocationID, path)
	if err != nil {
		return nil
	}
	return fileReference
}

func (fsh *StorageHandler) GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	result := make(map[string]interface{})
	result = fileref.GetListingData(ctx)

	if clientID != allocationObj.OwnerID {
		authTokenString := r.FormValue("auth_token")
		if len(authTokenString) == 0 {
			return nil, common.NewError("invalid_parameters", "Auth ticket required if data read by anyone other than owner.")
		}
		authToken := &readmarker.AuthTicket{}
		err = json.Unmarshal([]byte(authTokenString), &authToken)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
		}
		err = authToken.Verify(allocationObj, clientID)
		if err != nil {
			return nil, err
		}
		delete(result, "path")
	}
	return result, nil
}

func (fsh *StorageHandler) GetFileStats(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID{
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	result := make(map[string]interface{})
	result = fileref.GetListingData(ctx)
	stats, _ := stats.GetFileStats(ctx, fileref.ID)
	wm, _ := writemarker.GetWriteMarkerEntity(ctx, fileref.WriteMarker)
	if wm != nil && stats != nil {
		stats.WriteMarkerRedeemTxn = wm.CloseTxnID
	}
	var statsMap map[string]interface{}
	statsBytes, err := json.Marshal(stats)
	err = json.Unmarshal(statsBytes, &statsMap)
	for k, v := range statsMap {
		result[k] = v
	}
	return result, nil
}

func (fsh *StorageHandler) DownloadFile(ctx context.Context, r *http.Request) (*DownloadResponse, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	blockNumStr := r.FormValue("block_num")
	if len(blockNumStr) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNum, err := strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("invalid_parameters", "Invalid block number")
	}

	readMarkerString := r.FormValue("read_marker")
	readMarker := &readmarker.ReadMarker{}
	err = json.Unmarshal([]byte(readMarkerString), &readMarker)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the readmarker for download."+err.Error())
	}

	rmObj := &readmarker.ReadMarkerEntity{}
	rmObj.LatestRM = readMarker

	err = rmObj.VerifyMarker(ctx, allocationObj)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid read marker. Failed to verify the read marker. "+err.Error())
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file. "+err.Error())
	}

	if clientID != allocationObj.OwnerID {
		authTicketVerified, err := fsh.verifyAuthTicket(ctx, r, allocationObj, fileref, clientID)
		if err != nil {
			return nil, err
		}
		if !authTicketVerified {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
	}

	latestRM, err := readmarker.GetLatestReadMarker(ctx, clientID)
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return nil, common.NewError("read_marker_db_error", "Could not read from DB. "+err.Error())
	}

	if latestRM != nil && latestRM.ReadCounter+1 != readMarker.ReadCounter {
		//return nil, common.NewError("invalid_parameters", "Invalid read marker. Read counter was not for one block")
		response := &DownloadResponse{}
		response.Success = false
		response.LatestRM = latestRM
		response.Path = fileref.Path
		response.AllocationID = fileref.AllocationID
		return response, nil
	}

	download_mode := r.FormValue("content")
	var respData []byte
	if len(download_mode) > 0 && download_mode == DOWNLOAD_CONTENT_THUMB {
		fileData := &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ThumbnailHash
		respData, err = filestore.GetFileStore().GetFileBlock(allocationID, fileData, blockNum)
		if err != nil {
			return nil, err
		}
	} else {
		fileData := &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ContentHash
		respData, err = filestore.GetFileStore().GetFileBlock(allocationID, fileData, blockNum)
		if err != nil {
			return nil, err
		}
	}

	

	err = readmarker.SaveLatestReadMarker(ctx, readMarker, latestRM == nil)
	if err != nil {
		return nil, err
	}
	response := &DownloadResponse{}
	response.Success = true
	response.LatestRM = readMarker
	response.Data = respData
	response.Path = fileref.Path
	response.AllocationID = fileref.AllocationID
	
	stats.FileBlockDownloaded(ctx,fileref.ID)
	return response, nil
}

func (fsh *StorageHandler) ListEntities(ctx context.Context, r *http.Request) (*ListResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	Logger.Info("Path Hash for list dir :" + path_hash)

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}
	if clientID != allocationObj.OwnerID {
		authTicketVerified, err := fsh.verifyAuthTicket(ctx, r, allocationObj, fileref, clientID)
		if err != nil {
			return nil, err
		}
		if !authTicketVerified {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
	}

	dirref, err := reference.GetRefWithChildren(ctx, allocationID, fileref.Path)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}

	

	var result ListResult
	result.AllocationRoot = allocationObj.AllocationRoot
	result.Meta = dirref.GetListingData(ctx)
	if clientID != allocationObj.OwnerID {
		delete(result.Meta, "path")
	}
	result.Entities = make([]map[string]interface{}, len(dirref.Children))
	for idx, child := range dirref.Children {
		result.Entities[idx] = child.GetListingData(ctx)
		if clientID != allocationObj.OwnerID {
			delete(result.Entities[idx], "path")
		}
	}

	return &result, nil
}

func (fsh *StorageHandler) GetReferencePath(ctx context.Context, r *http.Request) (*ReferencePathResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	rootRef, err := reference.GetReferencePath(ctx, allocationID, path)
	if err != nil {
		return nil, err
	}

	refPath := &ReferencePath{ref: rootRef}
	refsToProcess := make([]*ReferencePath, 0)
	refsToProcess = append(refsToProcess, refPath)
	for len(refsToProcess) > 0 {
		refToProcess := refsToProcess[0]
		refToProcess.Meta = refToProcess.ref.GetListingData(ctx)
		if len(refToProcess.ref.Children) > 0 {
			refToProcess.List = make([]*ReferencePath, len(refToProcess.ref.Children))
		}
		for idx, child := range refToProcess.ref.Children {
			childRefPath := &ReferencePath{ref: child}
			refToProcess.List[idx] = childRefPath
			refsToProcess = append(refsToProcess, childRefPath)
		}
		refsToProcess = refsToProcess[1:]
	}

	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var refPathResult ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = &latestWM.WM
	}
	return &refPathResult, nil
}

func (fsh *StorageHandler) GetObjectPath(ctx context.Context, r *http.Request) (*ObjectPathResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNumStr := r.FormValue("block_num")
	if len(blockNumStr) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNum, err := strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("invalid_parameters", "Invalid block number")
	}

	objectPath, err := reference.GetObjectPath(ctx, allocationID, blockNum)
	if err != nil {
		return nil, err
	}

	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var objPathResult ObjectPathResult
	objPathResult.ObjectPath = objectPath
	if latestWM != nil {
		objPathResult.LatestWM = &latestWM.WM
	}
	return &objPathResult, nil
}

func (fsh *StorageHandler) CommitWrite(ctx context.Context, r *http.Request) (*CommitResult, error) {

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use POST instead")
	}
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	clientKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	clientKeyBytes, _ := hex.DecodeString(clientKey)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID || len(clientKey) == 0 || encryption.Hash(clientKeyBytes) != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	connectionID := r.FormValue("connection_id")
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	mutex := lock.GetMutex(allocationObj.TableName(), allocationID)
	mutex.Lock()
	defer mutex.Unlock()

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid connection id. Connection id was not found. "+err.Error())
	}
	if len(connectionObj.Changes) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id. Connection does not have any changes.")
	}

	if allocationObj.BlobberSizeUsed+connectionObj.Size > allocationObj.BlobberSize {
		return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	writeMarkerString := r.FormValue("write_marker")
	writeMarker := writemarker.WriteMarker{}
	err = json.Unmarshal([]byte(writeMarkerString), &writeMarker)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the writemarker for commit."+err.Error())
	}

	var result CommitResult
	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}

	writemarkerObj := &writemarker.WriteMarkerEntity{}
	writemarkerObj.WM = writeMarker

	err = writemarkerObj.VerifyMarker(ctx, allocationObj, connectionObj)
	if err != nil {
		result.AllocationRoot = allocationObj.AllocationRoot
		result.ErrorMessage = "Verification of write marker failed. " + err.Error()
		result.Success = false
		if latestWM != nil {
			result.WriteMarker = &latestWM.WM
		}
		return &result, common.NewError("write_marker_verification_failed", result.ErrorMessage)
	}

	err = connectionObj.ApplyChanges(ctx, writeMarker.AllocationRoot)
	if err != nil {
		return nil, err
	}
	rootRef, err := reference.GetReference(ctx, allocationID, "/")
	if err != nil {
		return nil, err
	}
	allocationRoot := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(writeMarker.Timestamp), 10))

	if allocationRoot != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWM != nil {
			result.WriteMarker = &latestWM.WM
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root. Expected hash: " + allocationRoot
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}
	writemarkerObj.ConnectionID = connectionObj.ConnectionID
	writemarkerObj.ClientPublicKey = clientKey
	err = writemarkerObj.Save(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	db := datastore.GetStore().GetTransaction(ctx)
	allocationUpdates := make(map[string]interface{})
	allocationUpdates["blobber_size_used"] = gorm.Expr("blobber_size_used + ?", connectionObj.Size)
	allocationUpdates["used_size"] = gorm.Expr("used_size + ?", connectionObj.Size)
	allocationUpdates["allocation_root"] = allocationRoot
	allocationUpdates["is_redeem_required"] = true

	err = db.Model(allocationObj).Updates(allocationUpdates).Error
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	err = connectionObj.CommitToFileStore(ctx)
	if err != nil {
		return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}

	result.Changes = connectionObj.Changes

	connectionObj.DeleteChanges(ctx)

	db.Model(connectionObj).Updates(allocation.AllocationChangeCollector{Status: allocation.CommittedConnection})

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = &writeMarker
	result.Success = true
	result.ErrorMessage = ""

	return &result, nil
}

func (fsh *StorageHandler) DeleteFile(ctx context.Context, r *http.Request, connectionObj *allocation.AllocationChangeCollector) (*UploadResult, error) {
	deleteTokenString := r.FormValue("delete_token")
	if len(deleteTokenString) == 0 {
		return nil, common.NewError("invalid_delete_token", "Invalid delete token. Blank values")
	}
	deleteToken := &writemarker.DeleteToken{}
	err := json.Unmarshal([]byte(deleteTokenString), deleteToken)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the delete token."+err.Error())
	}
	fileRef, _ := reference.GetReferenceFromLookupHash(ctx, connectionObj.AllocationID, deleteToken.FilePathHash)
	clientKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	if fileRef != nil && fileRef.Type == reference.FILE {
		deleteSize := fileRef.Size + fileRef.ThumbnailSize
		if deleteToken.AllocationID != connectionObj.AllocationID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Allocation ID mismatch.")
		}

		if deleteToken.BlobberID != node.Self.ID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Blobber ID mismatch.")
		}
		if deleteToken.ClientID != connectionObj.ClientID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Client ID mismatch.")
		}
		if deleteToken.Size != deleteSize {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Size mismatch.")
		}

		if deleteToken.FileRefHash != fileRef.Hash {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. File Ref hash mismatch.")
		}

		if deleteToken.FilePathHash != fileRef.PathHash {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. File Path hash mismatch.")
		}

		if !deleteToken.VerifySignature(clientKey) {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Signature verification failed.")
		}

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = 0 - deleteSize
		allocationChange.Operation = allocation.DELETE_OPERATION
		dfc := &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID, 
			AllocationID: connectionObj.AllocationID, Name: fileRef.Name, 
			Hash : fileRef.Hash, Path: fileRef.Path, Size: deleteSize, DeleteToken: deleteToken}
		
		connectionObj.Size += allocationChange.Size
		connectionObj.AddChange(allocationChange, dfc)
				
		result := &UploadResult{}
		result.Filename = fileRef.Name
		result.Hash = fileRef.Hash
		result.MerkleRoot = fileRef.MerkleRoot
		result.Size = fileRef.Size

		return result, nil
	}

	return nil, common.NewError("invalid_file", "File does not exist at path")
}

//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *StorageHandler) WriteFile(ctx context.Context, r *http.Request) (*UploadResult, error) {

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE instead")
	}

	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	connectionID := r.FormValue("connection_id")
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	result := &UploadResult{}
	mode := allocation.INSERT_OPERATION
	if r.Method == "PUT" {
		mode = allocation.UPDATE_OPERATION
	} else if r.Method == "DELETE" {
		mode = allocation.DELETE_OPERATION
	}
	if mode == allocation.DELETE_OPERATION {
		result, err = fsh.DeleteFile(ctx, r, connectionObj)
		if err != nil {
			return nil, err
		}
	} else if mode == allocation.INSERT_OPERATION || mode == allocation.UPDATE_OPERATION {
		var formData allocation.UpdateFileChange
		formField := "uploadMeta"
		if mode == allocation.UPDATE_OPERATION {
			formField = "updateMeta"
		}
		uploadMetaString := r.FormValue(formField)
		err = json.Unmarshal([]byte(uploadMetaString), &formData)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the meta data for upload."+err.Error())
		}
		exisitingFileRef := fsh.checkIfFileAlreadyExists(ctx, allocationID, formData.Path)
		existingFileRefSize := int64(0)
		if mode == allocation.INSERT_OPERATION && exisitingFileRef != nil {
			return nil, common.NewError("duplicate_file", "File at path already exists")
		} else if mode == allocation.UPDATE_OPERATION && exisitingFileRef == nil {
			return nil, common.NewError("invalid_file_update", "File at path does not exist for update")
		}

		if exisitingFileRef != nil {
			existingFileRefSize = exisitingFileRef.Size + exisitingFileRef.ThumbnailSize
		}

		origfile, _, err := r.FormFile("uploadFile")
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Error Reading multi parts for file." + err.Error())
		}
		defer origfile.Close()
		
		thumbfile, thumbHeader, _ := r.FormFile("uploadThumbnailFile")
		thumbnailPresent := false
		if thumbHeader != nil {
			thumbnailPresent = true
			defer thumbfile.Close()
		}
		
		fileInputData := &filestore.FileInputData{Name: formData.Filename, Path: formData.Path}
		fileOutputData, err := filestore.GetFileStore().WriteFile(allocationID, fileInputData, origfile, connectionObj.ConnectionID)
		if err != nil {
			return nil, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
		}
		

		result.Filename = formData.Filename
		result.Hash = fileOutputData.ContentHash
		result.MerkleRoot = fileOutputData.MerkleRoot
		result.Size = fileOutputData.Size

		if len(formData.Hash) > 0 && formData.Hash != fileOutputData.ContentHash {
			return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
		}
		if len(formData.MerkleRoot) > 0 && formData.MerkleRoot != fileOutputData.MerkleRoot {
			return nil, common.NewError("content_merkle_root_mismatch", "Merkle root provided in the meta data does not match the file content")
		}

		formData.Hash = fileOutputData.ContentHash
		formData.MerkleRoot = fileOutputData.MerkleRoot
		formData.AllocationID = allocationID
		formData.Size = fileOutputData.Size

		allocationSize := fileOutputData.Size
		if thumbnailPresent {
			thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: formData.Path}
			thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationID, thumbInputData, thumbfile, connectionObj.ConnectionID)
			if err != nil {
				return nil, common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
			}
			if len(formData.ThumbnailHash) > 0 && formData.ThumbnailHash != thumbOutputData.ContentHash {
				return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
			}
			formData.ThumbnailHash = thumbOutputData.ContentHash
			formData.ThumbnailSize = thumbOutputData.Size
			formData.ThumbnailFilename = thumbInputData.Name
			allocationSize += thumbOutputData.Size
		} 

		if allocationObj.BlobberSizeUsed+(allocationSize-existingFileRefSize) > allocationObj.BlobberSize {
			return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
		}

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = allocationSize - existingFileRefSize
		allocationChange.Operation = mode

		connectionObj.Size += allocationChange.Size
		if mode == allocation.INSERT_OPERATION {
			connectionObj.AddChange(allocationChange, &formData.NewFileChange)
		} else if (mode == allocation.UPDATE_OPERATION) {
			connectionObj.AddChange(allocationChange, &formData)
		}
	}
	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	return result, nil
}
