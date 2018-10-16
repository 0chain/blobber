package blobber

import (
	"encoding/json"

	"0chain.net/writemarker"

	"net/http"
	"os"

	"bytes"
	"fmt"

	"errors"

	"path/filepath"

	. "0chain.net/logging"
	"go.uber.org/zap"

	"0chain.net/common"
	"0chain.net/util"

	"strconv"
	"strings"
	"sync"
)

//ObjectStorageHandler - implments the StorageHandler interface
type ObjectStorageHandler struct {
	RootDirectory string
}

const (
	OSPathSeperator            string = string(os.PathSeparator)
	RefsDirName                       = "refs"
	ObjectsDirName                    = "objects"
	TempObjectsDirName                = "tmp"
	CurrentVersion                    = "1.0"
	FORM_FILE_PARSE_MAX_MEMORY        = 10 * 1024 * 1024
)

var mutex = &sync.Mutex{}

/*SetupFSStorageHandler - Setup a file system based block storage */
func SetupObjectStorageHandler(rootDir string) {
	util.CreateDirs(rootDir)
	SHandler = &ObjectStorageHandler{RootDirectory: rootDir}
}

func (fsh *ObjectStorageHandler) setupAllocation(allocationID string) (*Allocation, error) {
	allocation := &Allocation{ID: allocationID}
	allocation.Path = fsh.generateTransactionPath(allocationID)
	allocation.RefsPath = fmt.Sprintf("%s%s%s", allocation.Path, OSPathSeperator, RefsDirName)
	allocation.ObjectsPath = fmt.Sprintf("%s%s%s", allocation.Path, OSPathSeperator, ObjectsDirName)
	allocation.TempObjectsPath = filepath.Join(allocation.ObjectsPath, TempObjectsDirName)

	//create the allocation object dirs
	err := util.CreateDirs(allocation.ObjectsPath)
	if err != nil {
		Logger.Info("allocation_objects_dir_creation_error", zap.Any("allocation_objects_dir_creation_error", err))
		return nil, err
	}

	//create the allocation tmp object dirs
	err = util.CreateDirs(allocation.TempObjectsPath)
	if err != nil {
		Logger.Info("allocation_temp_objects_dir_creation_error", zap.Any("allocation_temp_objects_dir_creation_error", err))
		return nil, err
	}

	//create the allocation refs dirs
	err = util.CreateDirs(allocation.RefsPath)
	if err != nil {
		Logger.Info("allocation_refs_dir_creation_error", zap.Any("allocation_refs_dir_creation_error", err))
		return nil, err
	}

	root_ref, _ := allocation.getReferenceObject(OSPathSeperator, OSPathSeperator, true, true)

	if root_ref == nil {
		Logger.Info("allocation_refs_dir_creation_error", zap.Any("allocation_refs_dir_creation_error", err))
		return nil, errors.New("error loading the reference for root directory")
	}

	allocation.RootReferenceObject = *root_ref
	return allocation, nil
}

func (fsh *ObjectStorageHandler) generateTransactionPath(transID string) string {

	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s", fsh.RootDirectory, OSPathSeperator)
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", OSPathSeperator, transID[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", OSPathSeperator, transID[9:])
	return dir.String()
}

func (fsh *ObjectStorageHandler) ListEntities(r *http.Request, allocationID string) (*ListResponse, *common.Error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used for downloading the file. Use GET instead")
	}

	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("allocation_setup_error", err.Error())
	}

	file_path, ok := r.URL.Query()["path"]
	if !ok || len(file_path[0]) < 1 {
		return nil, common.NewError("invalid_parameters", "path parameter not found")
	}
	filePath := file_path[0]

	blobRefObject, _ := allocation.getReferenceObject(filePath, filePath, false, false)
	if blobRefObject == nil {
		Logger.Info("", zap.Any("blob_error", "Error getting the blob reference"))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("invalid_parameters", "Invalid path. Please check the parameters")
	}

	if blobRefObject.Header.ReferenceType != DIRECTORY {
		return nil, common.NewError("invalid_parameters", "Requested object is not a directory. Cannot list on directories. Please check the parameters")
	}

	blobRefObject.LoadReferenceEntries()

	response := &ListResponse{}

	//response.Name = blobRefObject.Hash
	response.ListEntries = make([]ListResponseEntity, 0)
	for i := range blobRefObject.RefEntries {
		var listEntry ListResponseEntity
		// Filename string `json:"filename"`
		// CustomMeta string `json:"custom_meta"`
		// Size int64 `json:"size"`
		// ContentHash string `json:"content_hash"`
		listEntry.Name = blobRefObject.RefEntries[i].Name
		listEntry.LookupHash = blobRefObject.RefEntries[i].LookupHash
		if blobRefObject.RefEntries[i].ReferenceType == DIRECTORY {
			listEntry.IsDir = true
		} else {
			listEntry.IsDir = false
		}
		response.ListEntries = append(response.ListEntries, listEntry)
	}
	return response, nil
}

func (fsh *ObjectStorageHandler) GetFileMeta(r *http.Request, allocationID string) (*FileMeta, *common.Error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used for downloading the file. Use GET instead")
	}

	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("allocation_setup_error", err.Error())
	}

	file_path, ok := r.URL.Query()["path"]
	if !ok || len(file_path[0]) < 1 {
		return nil, common.NewError("invalid_parameters", "path parameter not found")
	}
	filePath := file_path[0]

	filename, ok := r.URL.Query()["filename"]
	if !ok || len(filename[0]) < 1 {
		return nil, common.NewError("invalid_parameters", "path parameter not found")
	}
	fileName := filename[0]

	blobRefObject, _ := allocation.getReferenceObject(filepath.Join(filePath, fileName), fileName, false, false)
	if blobRefObject == nil {
		Logger.Info("", zap.Any("blob_error", "Error getting the blob reference"))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("invalid_parameters", "File not found. Please check the parameters")
	}

	if blobRefObject.Header.ReferenceType != FILE {
		return nil, common.NewError("invalid_parameters", "Requested object is not a file. Please check the parameters")
	}

	blobRefObject.LoadReferenceEntries()

	response := &FileMeta{}

	response.ID = blobRefObject.Hash
	response.Meta = make([]MetaInfo, 0)
	for i := range blobRefObject.RefEntries {
		var meta MetaInfo
		// Filename string `json:"filename"`
		// CustomMeta string `json:"custom_meta"`
		// Size int64 `json:"size"`
		// ContentHash string `json:"content_hash"`
		meta.Filename = blobRefObject.RefEntries[i].Name
		meta.CustomMeta = blobRefObject.RefEntries[i].CustomMeta
		meta.Size = blobRefObject.RefEntries[i].Size
		meta.ContentHash = blobRefObject.RefEntries[i].LookupHash
		response.Meta = append(response.Meta, meta)
	}
	return response, nil

}

func (fsh *ObjectStorageHandler) DownloadFile(r *http.Request, allocationID string) (*DownloadResponse, *common.Error) {
	if r.Method == "POST" {
		return nil, common.NewError("invalid_method", "Invalid method used for downloading the file. Use GET instead")
	}
	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("allocation_setup_error", err.Error())
	}

	file_path, ok := r.URL.Query()["path"]
	if !ok || len(file_path[0]) < 1 {
		return nil, common.NewError("invalid_parameters", "path parameter not found")
	}
	filePath := file_path[0]

	filename, ok := r.URL.Query()["filename"]
	if !ok || len(filename[0]) < 1 {
		return nil, common.NewError("invalid_parameters", "path parameter not found")
	}
	fileName := filename[0]

	blobRefObject, _ := allocation.getReferenceObject(filepath.Join(filePath, fileName), fileName, false, false)
	if blobRefObject == nil {
		Logger.Info("", zap.Any("blob_error", "Error getting the blob reference"))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("invalid_parameters", "File not found. Please check the parameters")
	}

	if blobRefObject.Header.ReferenceType != FILE {
		return nil, common.NewError("invalid_parameters", "Requested object is not a file. Please check the parameters")
	}

	blobRefObject.LoadReferenceEntries()

	part_hash, ok := r.URL.Query()["part_hash"]
	partHash := ""
	if !ok || len(part_hash[0]) < 1 {
		err = nil
	} else {
		partHash = part_hash[0]
	}

	if err != nil || (len(partHash) < 1) {
		return nil, common.NewError("invalid_parameters", "invalid part hash")
	}

	for i := range blobRefObject.RefEntries {
		if blobRefObject.RefEntries[i].LookupHash == partHash {
			partNum := i
			response := &DownloadResponse{}
			response.Filename = blobRefObject.RefEntries[partNum].Name
			response.Size = strconv.FormatInt(blobRefObject.RefEntries[partNum].Size, 10)
			dirPath, dirFileName := getFilePathFromHash(blobRefObject.RefEntries[partNum].LookupHash)
			response.Path = filepath.Join(allocation.ObjectsPath, dirPath, dirFileName)

			return response, nil
		}
	}

	return nil, common.NewError("invalid_parameters", "invalid part hash")
}

//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *ObjectStorageHandler) WriteFile(r *http.Request, allocationID string) UploadResponse {
	var response UploadResponse
	if r.Method == "GET" {
		return GenerateUploadResponseWithError(common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead"))
	}

	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("Error during setting up the allocation ", zap.Any("error", err))
		return GenerateUploadResponseWithError(common.NewError("allocation_setup_error", err.Error()))
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return GenerateUploadResponseWithError(common.NewError("request_parse_error", err.Error()))
	}

	response.Result = make([]UploadResult, 0)

	custom_meta := ""
	wm := &writemarker.WriteMarker{}
	data_id := ""
	for key, value := range r.MultipartForm.Value {
		if key == "custom_meta" {
			custom_meta = strings.Join(value, "")
		}
		if key == "write_marker" {
			wmString := strings.Join(value, "")
			Logger.Info("Write Marker", zap.Any("wm", wmString))
			err = json.Unmarshal([]byte(wmString), wm)
			if err != nil {
				Logger.Info("Invalid Write Marker in the request", zap.Any("error", err))
				return GenerateUploadResponseWithError(common.NewError("write_marker_decode_error", err.Error()))
			}
			data_id = wm.DataID
		}
	}

	if len(data_id) == 0 {
		return GenerateUploadResponseWithError(common.NewError("invalid_data_id", "No Data ID was found"))
	}

	protocolImpl := GetProtocolImpl(allocationID)

	err = protocolImpl.VerifyMarker(wm)
	if err != nil {
		return GenerateUploadResponseWithError(common.NewError("invalid_write_marker", err.Error()))
	}

	for _, fheaders := range r.MultipartForm.File {
		var result UploadResult
		for _, hdr := range fheaders {
			blobObject, common_error := allocation.writeFileAndCalculateHash(&allocation.RootReferenceObject, hdr, custom_meta, wm)
			if common_error != nil {
				result.Error = common_error
				result.Filename = hdr.Filename

			} else {
				result.Filename = blobObject.Filename
				result.Hash = blobObject.Hash
				result.Size = hdr.Size
			}
			response.Result = append(response.Result, result)
		}

	}

	return response
}
