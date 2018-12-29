package blobber

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"0chain.net/allocation"
	"0chain.net/readmarker"
	"0chain.net/writemarker"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/filestore"
	. "0chain.net/logging"
	"0chain.net/reference"
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

func (fsh *ObjectStorageHandler) verifyAllocation(ctx context.Context, allocationID string) (*allocation.Allocation, error) {
	if len(allocationID) == 0 {
		return nil, common.NewError("invalid_allocation", "Invalid allocation id")
	}
	allocationObj, err := GetProtocolImpl(allocationID).VerifyAllocationTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return allocationObj, nil
}

func (fsh *ObjectStorageHandler) DownloadFile(ctx context.Context, r *http.Request) (*DownloadResponse, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)

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
	rmEntity := readmarker.Provider().(*readmarker.ReadMarkerEntity)
	rmEntity.LatestRM = &readmarker.ReadMarker{}
	rmEntity.LatestRM.BlobberID = readMarker.BlobberID
	rmEntity.LatestRM.ClientID = readMarker.ClientID

	errRmRead := GetMetaDataStore().Read(ctx, rmEntity.GetKey(), rmEntity)
	if errRmRead != nil && errRmRead != datastore.ErrKeyNotFound {
		return nil, common.NewError("read_marker_db_error", "Could not read from DB. "+errRmRead.Error())
	}

	if rmEntity.LatestRM.ReadCounter+1 != readMarker.ReadCounter {
		return nil, common.NewError("invalid_parameters", "Invalid read marker. Read counter was not for one block")
	}

	rmEntity.LatestRM = readMarker

	fileref := reference.FileRefProvider().(*reference.FileRef)
	fileref.AllocationID = allocationID
	fileref.Path = path

	err = fileref.Read(ctx, fileref.GetKey())
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}
	fileData := &filestore.FileInputData{}
	fileData.Name = fileref.Name
	fileData.Path = fileref.Path
	fileData.Hash = fileref.ContentHash
	respData, err := fileStore.GetFileBlock(allocationID, fileData, blockNum)
	rmEntity.Write(ctx)
	response := &DownloadResponse{}
	response.Data = respData
	return response, err
}

func (fsh *ObjectStorageHandler) GetConnectionDetails(ctx context.Context, r *http.Request) (*AllocationChangeCollector, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID)
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

	connectionObj := AllocationChangeCollectorProvider().(*AllocationChangeCollector)
	connectionObj.ConnectionID = connectionID
	connectionObj.AllocationID = allocationID

	err = GetMetaDataStore().Read(ctx, connectionObj.GetKey(), connectionObj)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid connection id. Connection id was not found. "+err.Error())
	}

	return connectionObj, nil
}

func (fsh *ObjectStorageHandler) GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	_, err := fsh.verifyAllocation(ctx, allocationID)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	fileref := reference.FileRefProvider().(*reference.FileRef)
	fileref.AllocationID = allocationID
	fileref.Path = path

	err = fileref.Read(ctx, fileref.GetKey())
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path / file. "+err.Error())
	}

	result := make(map[string]interface{})
	result = fileref.GetListingData(ctx)
	return result, nil
}

func (fsh *ObjectStorageHandler) ListEntities(ctx context.Context, r *http.Request) (*ListResult, error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationID)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
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

	err = dirref.LoadChildren(ctx)
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
	//clientKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)

	mutex := GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()

	allocationObj, err := fsh.verifyAllocation(ctx, allocationID)

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

	connectionObj := AllocationChangeCollectorProvider().(*AllocationChangeCollector)
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

	rootRef, err := connectionObj.ApplyChanges(ctx)
	if err != nil {
		return nil, err
	}

	if rootRef.Hash != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWM != nil {
			result.WriteMarker = latestWM.WM
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root. Expected hash: " + rootRef.Hash
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}

	writemarkerObj := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	if latestWM != nil {
		writemarkerObj.PrevWM = latestWM.GetKey()
	}

	writemarkerObj.WM = writeMarker
	err = writemarkerObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	allocationObj.BlobberSizeUsed += connectionObj.Size
	allocationObj.UsedSize += connectionObj.Size
	allocationObj.AllocationRoot = rootRef.Hash
	allocationObj.LatestWMEntity = writemarkerObj.GetKey()
	err = allocationObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	connectionObj.Delete(ctx)

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = writeMarker
	result.Success = true
	result.ErrorMessage = ""
	return &result, nil
}

func (fsh *ObjectStorageHandler) checkIfFileAlreadyExists(ctx context.Context, allocationID string, path string) bool {
	mutex := GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()
	fileReference := reference.FileRefProvider().(*reference.FileRef)
	fileReference.AllocationID = allocationID
	fileReference.Path = path
	err := GetMetaDataStore().Read(ctx, fileReference.GetKey(), fileReference)
	if err != nil && err == datastore.ErrKeyNotFound {
		return false
	}
	return true
}

func (fsh *ObjectStorageHandler) checkIfFilePartOfExisitingOpenConnection(ctx context.Context, allocationID string, path string) bool {
	connectionObj := AllocationChangeCollectorProvider()
	mutex := GetMutex(allocationID)
	mutex.Lock()
	defer mutex.Unlock()
	handler := func(ctx context.Context, key datastore.Key, value []byte) error {
		connectionObj := AllocationChangeCollectorProvider().(*AllocationChangeCollector)
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

//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *ObjectStorageHandler) WriteFile(ctx context.Context, r *http.Request) (*UploadResult, error) {
	var result UploadResult
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead")
	}

	allocationID := ctx.Value(ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	//clientKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationID)

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
	fmt.Println(uploadMetaString)
	var formData UploadFormData
	err = json.Unmarshal([]byte(uploadMetaString), &formData)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if len(formData.ConnectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	if fsh.checkIfFileAlreadyExists(ctx, allocationID, formData.Path) {
		return nil, common.NewError("duplicate_file", "File at path already exists")
	}

	if fsh.checkIfFilePartOfExisitingOpenConnection(ctx, allocationID, formData.Path) {
		return nil, common.NewError("duplicate_file", "File at path already uploaded as part of another open connection")
	}

	connectionObj := AllocationChangeCollectorProvider().(*AllocationChangeCollector)
	connectionObj.ConnectionID = formData.ConnectionID
	connectionObj.AllocationID = allocationID
	connectionObj.ClientID = clientID

	mutex := GetMutex(connectionObj.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err = GetMetaDataStore().Read(ctx, connectionObj.GetKey(), connectionObj)
	if err != nil && err != datastore.ErrKeyNotFound {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

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

			if allocationObj.BlobberSizeUsed+fileOutputData.Size > allocationObj.BlobberSize {
				return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
			}

			formData.Hash = fileOutputData.ContentHash
			formData.MerkleRoot = fileOutputData.MerkleRoot

			allocationChange := &AllocationChange{}
			allocationChange.Size = fileOutputData.Size
			allocationChange.UploadFormData = &formData
			allocationChange.Operation = INSERT_OPERATION

			connectionObj.Size += fileOutputData.Size
			connectionObj.AddChange(allocationChange)
		}

	}
	connectionObj.LastUpdated = common.Now()
	err = connectionObj.Write(ctx)
	if err != nil {
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	return &result, nil
}
