package blobber

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"0chain.net/allocation"
	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/filestore"
	"0chain.net/hashmapstore"
	"0chain.net/lock"
	. "0chain.net/logging"
	"0chain.net/node"
	"0chain.net/readmarker"
	"0chain.net/reference"
	"0chain.net/stats"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

const FORM_FILE_PARSE_MAX_MEMORY = 10 * 1024 * 1024

//ObjectStorageHandler - implments the StorageHandler interface
type ObjectStorageHandler struct {
}

/*SetupFSStorageHandler - Setup a file system based block storage */
func SetupObjectStorageHandler(fsStore filestore.FileStore, metaStore datastore.Store) {
	SHandler = &ObjectStorageHandler{}
	metaDataStore = metaStore
	fileStore = fsStore
}

func (fsh *ObjectStorageHandler) verifyAllocation(ctx context.Context, allocationID string, readonly bool) (*allocation.Allocation, error) {
	if len(allocationID) == 0 {
		return nil, common.NewError("invalid_allocation", "Invalid allocation id")
	}
	allocationObj, err := GetProtocolImpl(allocationID).VerifyAllocationTransaction(ctx, readonly)
	if err != nil {
		return nil, err
	}
	return allocationObj, nil
}

func (fsh *ObjectStorageHandler) AcceptChallenge(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	challengeID := r.FormValue("challenge_txn")
	if len(challengeID) == 0 {
		return nil, common.NewError("invalid_parameters", "No challenge txn passed")
	}

	retObj := make(map[string]string)
	retObj["id"] = challengeID
	retObj["status"] = "Accepted"
	return retObj, nil
}

func (fsh *ObjectStorageHandler) GetLatestReadMarker(ctx context.Context, r *http.Request) (*readmarker.ReadMarker, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	rmEntity := readmarker.Provider().(*readmarker.ReadMarkerEntity)
	err := rmEntity.GetLatestReadMarker(ctx, clientID, node.Self.ID)
	if err != nil {
		return nil, err
	}
	return rmEntity.LatestRM, nil
}

func (fsh *ObjectStorageHandler) DownloadFile(ctx context.Context, r *http.Request) (*DownloadResponse, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	_ = ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)

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

	err = GetProtocolImpl(allocationID).VerifyReadMarker(ctx, readMarker, allocationObj)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid read marker. Failed to verify the read marker. "+err.Error())
	}

	fileref := reference.FileRefProvider().(*reference.FileRef)

	err = fileref.Read(ctx, fileref.GetKeyFromPathHash(path_hash))
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}

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
		err = authToken.Verify(allocationObj, fileref.Name, fileref.PathHash, clientID)
		if err != nil {
			return nil, err
		}
	}

	rmEntity := readmarker.Provider().(*readmarker.ReadMarkerEntity)
	errRmRead := rmEntity.GetLatestReadMarker(ctx, clientID, node.Self.ID)
	if errRmRead != nil && errRmRead != datastore.ErrKeyNotFound {
		return nil, common.NewError("read_marker_db_error", "Could not read from DB. "+errRmRead.Error())
	}

	if rmEntity.LatestRM.ReadCounter+1 != readMarker.ReadCounter {
		//return nil, common.NewError("invalid_parameters", "Invalid read marker. Read counter was not for one block")
		response := &DownloadResponse{}
		response.Success = false
		response.LatestRM = rmEntity.LatestRM
		response.Path = fileref.Path
		response.AllocationID = fileref.AllocationID
		return response, nil
	}

	rmEntity.LatestRM = readMarker

	fileData := &filestore.FileInputData{}
	fileData.Name = fileref.Name
	fileData.Path = fileref.Path
	fileData.Hash = fileref.ContentHash
	respData, err := fileStore.GetFileBlock(allocationID, fileData, blockNum)
	if err != nil {
		return nil, err
	}

	err = rmEntity.Write(ctx)
	if err != nil {
		return nil, err
	}
	response := &DownloadResponse{}
	response.Success = true
	response.LatestRM = readMarker
	response.Data = respData
	response.Path = fileref.Path
	response.AllocationID = fileref.AllocationID
	return response, nil
}

func (fsh *ObjectStorageHandler) GetConnectionDetails(ctx context.Context, r *http.Request) (*allocation.AllocationChangeCollector, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	connectionID := r.FormValue("connection_id")
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
	connectionObj.ConnectionID = connectionID
	connectionObj.AllocationID = allocationID

	err = GetMetaDataStore().Read(ctx, connectionObj.GetKey(), connectionObj)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid connection id. Connection id was not found. "+err.Error())
	}

	return connectionObj, nil
}

func (fsh *ObjectStorageHandler) GetFileStats(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method != "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	_ = ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref := reference.FileRefProvider().(*reference.FileRef)

	err = fileref.Read(ctx, fileref.GetKeyFromPathHash(path_hash))
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path / file. "+err.Error())
	}

	filestats, err := stats.GetFileStats(ctx, path_hash)

	if err != nil {
		return nil, common.NewError("no_data", "No Stats for the file")
	}

	wm := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	wm.Read(ctx, filestats.WriteMarker)
	if wm.Status == writemarker.Committed {
		filestats.WriteMarkerRedeemTxn = wm.CloseTxnID
	}

	result := make(map[string]interface{})
	result["meta"] = fileref.GetListingData(ctx)
	result["stats"] = filestats

	return result, nil
}

func (fsh *ObjectStorageHandler) GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	_ = ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)

	path_hash := r.FormValue("path_hash")
	path := r.FormValue("path")
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref := reference.FileRefProvider().(*reference.FileRef)

	err = fileref.Read(ctx, fileref.GetKeyFromPathHash(path_hash))
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path / file. "+err.Error())
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
		err = authToken.Verify(allocationObj, fileref.Name, fileref.PathHash, clientID)
		if err != nil {
			return nil, err
		}
		delete(result, "path")
	}
	return result, nil
}

func (fsh *ObjectStorageHandler) GetReferencePath(ctx context.Context, r *http.Request) (*ReferencePathResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	refPath, err := reference.GetReferencePath(ctx, allocationID, path, metaDataStore)
	if err != nil {
		return nil, err
	}

	latestWM := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	if len(allocationObj.LatestWMEntity) == 0 {
		latestWM = nil
	} else {
		err = latestWM.Read(ctx, allocationObj.LatestWMEntity)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var refPathResult ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = latestWM.WM
	}
	return &refPathResult, nil
}

func (fsh *ObjectStorageHandler) GetObjectPathFromBlockNum(ctx context.Context, r *http.Request) (*ObjectPathResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	blockNumStr := r.FormValue("block_num")
	if len(blockNumStr) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNum, err := strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("invalid_parameters", "Invalid block number")
	}

	allocationRoot := r.FormValue("allocation_root")
	if len(allocationRoot) == 0 {
		//use latest allocation root from the allocation object
		allocationRoot = allocationObj.AllocationRoot
	}

	wm := *writemarker.Provider().(*writemarker.WriteMarkerEntity)
	writeMarker := &writemarker.WriteMarker{}
	writeMarker.AllocationID = allocationID
	writeMarker.AllocationRoot = allocationRoot
	wm.WM = writeMarker
	err = wm.Read(ctx, wm.GetKey())
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation root. No write marker found")
	}

	if wm.DirStructure == nil {
		wm.WriteAllocationDirStructure(ctx)
	}

	dbStore := hashmapstore.NewStore()
	dbStore.DB = wm.DirStructure

	objectpath, err := reference.GetObjectPath(ctx, allocationID, blockNum, dbStore)

	var retObj ObjectPathResult
	retObj.ObjectPath = objectpath
	retObj.AllocationRoot = wm.WM.AllocationRoot

	return &retObj, nil
}

func (fsh *ObjectStorageHandler) ListEntities(ctx context.Context, r *http.Request) (*ListResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	dirref := reference.RefProvider().(*reference.Ref)
	dirref.AllocationID = allocationID
	dirref.Path = path

	err = dirref.Read(ctx, dirref.GetKey())
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}

	err = dirref.LoadChildren(ctx, dirref.GetEntityMetadata().GetStore())
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Error loading children in path. "+err.Error())
	}

	var result ListResult
	result.AllocationRoot = allocationObj.AllocationRoot
	result.Meta = dirref.GetListingData(ctx)
	result.Entities = make([]map[string]interface{}, len(dirref.ChildRefs))
	for idx, child := range dirref.Children {
		result.Entities[idx] = child.GetListingData(ctx)
	}

	return &result, nil
}

func (fsh *ObjectStorageHandler) CommitWrite(ctx context.Context, r *http.Request) (*CommitResult, error) {

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use POST instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	clientKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)
	clientKeyBytes, _ := hex.DecodeString(clientKey)

	mutex := lock.GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()

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

	connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
	connectionObj.ConnectionID = connectionID
	connectionObj.AllocationID = allocationID

	err = GetMetaDataStore().Read(ctx, connectionObj.GetKey(), connectionObj)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid connection id. Connection id was not found. "+err.Error())
	}

	if allocationObj.BlobberSizeUsed+connectionObj.Size > allocationObj.BlobberSize {
		return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	writeMarkerString := r.FormValue("write_marker")
	writeMarker := &writemarker.WriteMarker{}
	err = json.Unmarshal([]byte(writeMarkerString), &writeMarker)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the writemarker for commit."+err.Error())
	}

	var result CommitResult
	latestWM := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	if len(allocationObj.LatestWMEntity) == 0 {
		latestWM = nil
	} else {
		err = latestWM.Read(ctx, allocationObj.LatestWMEntity)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}

	writemarkerObj := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	if latestWM != nil {
		writemarkerObj.PrevWM = latestWM.GetKey()
	}

	writemarkerObj.WM = writeMarker
	writemarkerObj.Changes = connectionObj.Changes

	err = GetProtocolImpl(allocationID).VerifyMarker(ctx, writeMarker, allocationObj, connectionObj)
	if err != nil {
		result.AllocationRoot = allocationObj.AllocationRoot
		result.ErrorMessage = "Verification of write marker failed. " + err.Error()
		result.Success = false
		if latestWM != nil {
			result.WriteMarker = latestWM.WM
		}
		return &result, common.NewError("write_marker_verification_failed", result.ErrorMessage)
	}

	rootRef, err := connectionObj.ApplyChanges(ctx, fileStore, nil, writemarkerObj.GetKey())
	if err != nil {
		return nil, err
	}
	allocationRoot := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(writeMarker.Timestamp), 10))

	if allocationRoot != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWM != nil {
			result.WriteMarker = latestWM.WM
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root. Expected hash: " + allocationRoot
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}
	writemarkerObj.ClientPublicKey = clientKey
	err = writemarkerObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	allocationObj.BlobberSizeUsed += connectionObj.Size
	allocationObj.UsedSize += connectionObj.Size
	allocationObj.AllocationRoot = allocationRoot
	allocationObj.LatestWMEntity = writemarkerObj.GetKey()
	err = allocationObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	err = connectionObj.CommitToFileStore(ctx, fileStore)
	if err != nil {
		return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}

	result.Changes = connectionObj.Changes

	connectionObj.DeleteChanges(ctx, fileStore)

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = writeMarker
	result.Success = true
	result.ErrorMessage = ""

	return &result, nil
}

func (fsh *ObjectStorageHandler) checkIfFileAlreadyExists(ctx context.Context, allocationID string, path string) *reference.FileRef {
	mutex := lock.GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()
	fileReference := reference.FileRefProvider().(*reference.FileRef)
	fileReference.AllocationID = allocationID
	fileReference.Path = path
	err := GetMetaDataStore().Read(ctx, fileReference.GetKey(), fileReference)
	if err != nil && err == datastore.ErrKeyNotFound {
		return nil
	}
	return fileReference
}

func (fsh *ObjectStorageHandler) checkIfFilePartOfExisitingOpenConnection(ctx context.Context, allocationID string, path string) bool {
	connectionObj := allocation.AllocationChangeCollectorProvider()
	mutex := lock.GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()
	handler := func(ctx context.Context, key datastore.Key, value []byte) error {
		connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
		err := json.Unmarshal(value, connectionObj)
		if err != nil {
			return err
		}
		connectionObj.ComputeChangeMap()
		refKey := reference.GetReferenceLookup(allocationID, path)
		if _, ok := connectionObj.ChangeMap[refKey]; ok {
			Logger.Info("File already exists as part of an open connection")
			return common.NewError("duplicate_file", "File already uploaded as part of exisiting open connection")
		}

		return nil
	}

	err := GetMetaDataStore().IteratePrefix(ctx, connectionObj.GetEntityMetadata().GetDBName(), handler)

	if err != nil {
		return true
	}
	return false
}

func (fsh *ObjectStorageHandler) DeleteFile(ctx context.Context, r *http.Request, formData *allocation.UploadFormData, connectionObj *allocation.AllocationChangeCollector) (*UploadResult, error) {

	fileRef := fsh.checkIfFileAlreadyExists(ctx, connectionObj.AllocationID, formData.Path)
	clientKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)
	if fileRef != nil {
		deleteTokenString := r.FormValue("delete_token")
		//fmt.Println(uploadMetaString)
		deleteToken := allocation.DeleteTokenProvider().(*allocation.DeleteToken)
		err := json.Unmarshal([]byte(deleteTokenString), deleteToken)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the delete token."+err.Error())
		}

		if deleteToken.AllocationID != connectionObj.AllocationID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Allocation ID mismatch.")
		}

		if deleteToken.BlobberID != node.Self.ID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Blobber ID mismatch.")
		}
		if deleteToken.ClientID != connectionObj.ClientID {
			return nil, common.NewError("invalid_delete_token", "Invalid delete token. Client ID mismatch.")
		}
		if deleteToken.Size != fileRef.Size {
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
		allocationChange.Size = fileRef.Size
		allocationChange.UploadFormData = formData
		allocationChange.Operation = allocation.DELETE_OPERATION
		allocationChange.NumBlocks = int64(math.Ceil(float64(allocationChange.Size*1.0) / reference.CHUNK_SIZE))
		deleteToken.Status = allocation.NEW
		allocationChange.DeleteToken = deleteToken.GetKey()
		err = deleteToken.Write(ctx)
		if err != nil {
			return nil, common.NewError("delete_token_write_failed", "Delete token failed to save."+err.Error())
		}
		connectionObj.Size -= allocationChange.Size
		connectionObj.AddChange(allocationChange)
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
func (fsh *ObjectStorageHandler) WriteFile(ctx context.Context, r *http.Request) (*UploadResult, error) {

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE instead")
	}

	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)

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

	uploadMetaString := r.FormValue("uploadMeta")
	//fmt.Println(uploadMetaString)
	var formData allocation.UploadFormData
	err = json.Unmarshal([]byte(uploadMetaString), &formData)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if len(formData.ConnectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
	connectionObj.ConnectionID = formData.ConnectionID
	connectionObj.AllocationID = allocationID
	connectionObj.ClientID = clientID

	mutex := lock.GetMutex(connectionObj.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err = GetMetaDataStore().Read(ctx, connectionObj.GetKey(), connectionObj)
	if err != nil && err != datastore.ErrKeyNotFound {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}
	result := &UploadResult{}
	mode := allocation.INSERT_OPERATION
	if r.Method == "PUT" {
		mode = allocation.UPDATE_OPERATION
	} else if r.Method == "DELETE" {
		mode = allocation.DELETE_OPERATION
	}
	if mode == allocation.DELETE_OPERATION {
		result, err = fsh.DeleteFile(ctx, r, &formData, connectionObj)
		if err != nil {
			return nil, err
		}
	} else if mode == allocation.INSERT_OPERATION || mode == allocation.UPDATE_OPERATION {
		exisitingFileRef := fsh.checkIfFileAlreadyExists(ctx, allocationID, formData.Path)
		existingFileRefSize := int64(0)
		if mode == allocation.INSERT_OPERATION && exisitingFileRef != nil {
			return nil, common.NewError("duplicate_file", "File at path already exists")
		} else if mode == allocation.UPDATE_OPERATION && exisitingFileRef == nil {
			return nil, common.NewError("invalid_file_update", "File at path does not exist for update")
		}

		if exisitingFileRef != nil {
			existingFileRefSize = exisitingFileRef.Size
		}

		// if fsh.checkIfFilePartOfExisitingOpenConnection(ctx, allocationID, formData.Path) {
		// 	return nil, common.NewError("duplicate_file", "File at path already uploaded as part of another open connection")
		// }

		for _, fheaders := range r.MultipartForm.File {
			for _, hdr := range fheaders {
				fileInputData := &filestore.FileInputData{Name: formData.Filename, Path: formData.Path}
				fileOutputData, err := fileStore.WriteFile(allocationID, fileInputData, hdr, connectionObj.ConnectionID)
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

				if allocationObj.BlobberSizeUsed+(fileOutputData.Size-existingFileRefSize) > allocationObj.BlobberSize {
					return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
				}

				formData.Hash = fileOutputData.ContentHash
				formData.MerkleRoot = fileOutputData.MerkleRoot

				allocationChange := &allocation.AllocationChange{}
				allocationChange.Size = fileOutputData.Size - existingFileRefSize
				allocationChange.NewFileSize = fileOutputData.Size
				allocationChange.UploadFormData = &formData
				allocationChange.Operation = mode
				allocationChange.NumBlocks = int64(math.Ceil(float64(allocationChange.Size*1.0) / reference.CHUNK_SIZE))

				connectionObj.Size += allocationChange.Size
				connectionObj.AddChange(allocationChange)
			}

		}
	}

	connectionObj.LastUpdated = common.Now()
	err = connectionObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	return result, nil
}
