package blobber

import (
	
	"net/http"
	"os"
	
	"fmt"
	"bytes"
	"strings"
	"bufio"
	"mime/multipart"
	"io"
	"errors"

	. "0chain.net/logging"
	"go.uber.org/zap"
	"path/filepath"

	"0chain.net/common"
	"0chain.net/encryption"
	"0chain.net/util"
	"crypto/sha1"
	"encoding/hex"
	"github.com/jszwec/csvutil"
	"encoding/csv"
	"strconv"
)

//ObjectStorageHandler - implments the StorageHandler interface
type ObjectStorageHandler struct {
	RootDirectory string
}

const (
	OSPathSeperator string = string(os.PathSeparator)
	RefsDirName = "refs"
	ObjectsDirName = "objects"
	TempObjectsDirName = "tmp"
	CurrentVersion = "1.0"
	FORM_FILE_PARSE_MAX_MEMORY = 10 * 1024 * 1024
)

type Allocation struct {
	ID string
	Path string
	ObjectsPath string
	TempObjectsPath string
	RefsPath string 
	RootReferenceObject ReferenceObject
}

type ReferenceHeader struct {
	Version string
	ReferenceType EntryType
}

type ReferenceEntry struct {
	ReferenceType EntryType `csv:"type"`
	Name string `csv:"name"`
	LookupHash string `csv:"lookup_hash"`
	PreviousRevisionHash string `csv:"previous_rev_hash"`
	Size uint64 `csv:"size"`
	IsCompressed bool `csv:"is_compressed"`	
}

type ReferenceObject struct {
	ID string
	Hash string
	Path string
	Filename string
	FullPath string
	ActualPath string
	ActualFilename string
	Header ReferenceHeader
	RefEntries []ReferenceEntry 
}

type BlobObject struct {
	ID string
	Hash string
	FilenameHash string
	Path string
	ActualPath string
	Filename string
	Ref *ReferenceObject  
}

type EntryType int

const (
	FILE EntryType = 1 + iota
	DIRECTORY 
)

func (e EntryType) String() string{
	switch(e) {
		case FILE : 
			return "f"
		case DIRECTORY :
			return "d"	
	}
	return ""
}

func ParseEntryType(s string) EntryType {
	if(s == "f"){
		return FILE
	} else if(s == "d"){
		return DIRECTORY
	}
	return -1
}


/*SetupFSStorageHandler - Setup a file system based block storage */
func SetupObjectStorageHandler(rootDir string) {
	util.CreateDirs(rootDir)
	SHandler = &ObjectStorageHandler{RootDirectory: rootDir}
}

func getFilePathFromHash(hash string) (string, string) {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s", hash[0:3])
	for i := 1; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), hash[3*i:3*i+3])
	}
	return dir.String(), hash[9:]
}


func (allocation * Allocation) getReferenceObject(relativePath string, filename string, isDir bool, shouldCreateRef bool) *ReferenceObject {
	refObject := &ReferenceObject{ActualPath: relativePath, ActualFilename : filename}
	refObject.Hash = util.Hash(refObject.ActualPath)
	refObject.ID = fmt.Sprintf("%s%s%s", allocation.ID, "-", refObject.Hash)
	path, filename := getFilePathFromHash(refObject.Hash);
	refObject.Path = filepath.Join(allocation.RefsPath, path)
	refObject.Filename = filename
	refObject.FullPath = filepath.Join(refObject.Path, refObject.Filename)

	//create the root reference dirs
	err := util.CreateDirs(refObject.Path)
	if err != nil {
		Logger.Info("reference_dir_creation_error", zap.Any("reference_dir_creation_error", err))
		return nil
	}

	if _, err := os.Stat(refObject.FullPath); err != nil {
		var fh *os.File;
	    if os.IsNotExist(err) {
	    	if !shouldCreateRef {
	    		return nil;
	    	}
	        //create the root reference file
			fh, err = os.Create(refObject.FullPath)
			if err != nil {
				Logger.Info("reference_file_creation_error", zap.Any("reference_file_creation_error", err))
				return nil
			}
			defer fh.Close()
			refObject.Header.Version = CurrentVersion
			if isDir {
				refObject.Header.ReferenceType = DIRECTORY
			} else {
				refObject.Header.ReferenceType = FILE
			}
			w := bufio.NewWriter(fh)
			w.WriteString(strings.Join(refObject.GetHeaders(), ",") + "\n")
			w.Flush()
	    }
	} else {
    	fh, err := os.Open(refObject.FullPath)
		if err != nil {
			Logger.Info("reference_file_open_error", zap.Any("reference_file_open_error", err))
			return nil
		}
		defer fh.Close()
		r:= bufio.NewReader(fh)
		header,_ := r.ReadString('\n')
		header = strings.TrimSuffix(header, "\n")
		Logger.Info("", zap.Any("header", header))
		refObject.LoadHeader(strings.Split(header, ","))
    }

	return refObject
}

func (allocation *Allocation) writeFileAndCalculateHash(parentRef *ReferenceObject, fileHeader *multipart.FileHeader) (*BlobObject, *common.Error) {
	blobRefObject := allocation.getReferenceObject(filepath.Join(parentRef.ActualPath, fileHeader.Filename), fileHeader.Filename, false, true)
	
	blobObject := &BlobObject{Filename : fileHeader.Filename, Ref: blobRefObject}

	h := sha1.New()
	tempFilePath := filepath.Join(allocation.TempObjectsPath, blobObject.Filename + "." + encryption.Hash(blobObject.Ref.ActualPath))
    dest, err := os.Create(tempFilePath)
    if err != nil {
        return nil, common.NewError("file_creation_error", err.Error())
    }
    defer dest.Close()
    infile, err := fileHeader.Open()
    if err != nil {
        return nil, common.NewError("file_reading_error", err.Error())
    }
    tReader := io.TeeReader(infile, h)
    _, err = io.Copy(dest, tReader)
    if err != nil {
        return nil, common.NewError("file_write_error", err.Error())
    }
    blobObject.Hash = hex.EncodeToString(h.Sum(nil))
    Logger.Info("blob_hash", zap.Any("blob_hash", blobObject.Hash))

    //move file from tmp location to the objects folder
    dirPath, destFile := getFilePathFromHash(blobObject.Hash)
	blobObject.Path = filepath.Join(allocation.ObjectsPath, dirPath)
    err = util.CreateDirs(blobObject.Path);
    if err != nil {
        return nil, common.NewError("blob_object_dir_creation_error", err.Error())
    }
    blobObject.Path = filepath.Join(blobObject.Path, destFile)
    err =  os.Rename(tempFilePath, blobObject.Path)

	if err != nil {
	   return nil, common.NewError("blob_object_creation_error", err.Error())
	}

	err = parentRef.AppendReferenceEntry(&ReferenceEntry{ReferenceType : FILE, Name: blobObject.Filename, LookupHash: blobObject.Hash})
	if err != nil {
	   return nil, common.NewError("reference_parent_append_error", err.Error())
	}

	err = blobRefObject.AppendReferenceEntry(&ReferenceEntry{ReferenceType : FILE, Name: blobObject.Filename, LookupHash: blobObject.Hash})
	if err != nil {
	   return nil, common.NewError("reference_object_append_error", err.Error())
	}

    return blobObject, nil
}

func (refObject *ReferenceObject) AppendReferenceEntry(entry *ReferenceEntry) (error) {
	
	fh, err := os.OpenFile(refObject.FullPath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		Logger.Info("reference_object_open_error", zap.Any("reference_object_open_error", err))
		return err
	}
	defer fh.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	enc := csvutil.NewEncoder(w)
	enc.AutoHeader = false
	enc.Encode(entry)
	w.Flush()

	if _, err = fh.WriteString(buf.String()); err != nil {
	    return err
	}
	return nil

}

func (refObject *ReferenceObject) LoadReferenceEntries() (error){
	fh, err := os.Open(refObject.FullPath)
	if err != nil {
		Logger.Info("reference_object_open_error", zap.Any("reference_object_open_error", err))
		return err
	}
	defer fh.Close()
	
	r:= bufio.NewReader(fh)
	r.ReadString('\n')
	
	csvReader := csv.NewReader(r)
	// ReferenceType EntryType `csv:"type"`
	// Name string `csv:"name"`
	// LookupHash string `csv:"lookup_hash"`
	// PreviousRevisionHash string `csv:"previous_rev_hash"`
	// Size uint64 `csv:"size"`
	// IsCompressed bool `csv:"is_compressed"`	

	dec, err := csvutil.NewDecoder(csvReader, "type", "name", "lookup_hash", "previous_rev_hash", "size", "is_compressed")
	if(err != nil) {
		Logger.Info("reference_object_decode_error", zap.Any("reference_object_decode_error", err))
		return err
	}

	refObject.RefEntries = make([]ReferenceEntry, 0)

	for {
		u := ReferenceEntry{}
		if err := dec.Decode(&u); err == io.EOF {
			break
		} else if err != nil {
			Logger.Info("reference_decode_error", zap.Any("reference_decode_error", err))
			return err
		}

		refObject.RefEntries = append(refObject.RefEntries, u)
	}
	Logger.Info("ref_entries", zap.Any("ref_entries", len(refObject.RefEntries)))

	return nil
}

func (refObject *ReferenceObject) GetHeaders() ([]string) {
	return []string{refObject.Header.Version, refObject.Header.ReferenceType.String()};
}

func (refObject *ReferenceObject) LoadHeader(headers []string) {
	if(len(headers) > 0) { 
		refObject.Header.Version = headers[0]
		refObject.Header.ReferenceType = ParseEntryType(headers[1])
	}
} 


func (fsh *ObjectStorageHandler) setupAllocation(allocationID string) (*Allocation, error){
	allocation:= &Allocation{ID: allocationID}
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

	root_ref := allocation.getReferenceObject(OSPathSeperator, OSPathSeperator, true, true)

	if(root_ref == nil) {
		Logger.Info("allocation_refs_dir_creation_error", zap.Any("allocation_refs_dir_creation_error", err))
		return nil, errors.New("error loading the reference for root directory")
	}

	allocation.RootReferenceObject = *root_ref
	Logger.Info("", zap.Any("ref.Version", root_ref.Header.Version))
	Logger.Info("", zap.Any("ref.Type", root_ref.Header.ReferenceType))
	return allocation, nil
}



func (fsh *ObjectStorageHandler) generateTransactionPath(transID string) string{

	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s", fsh.RootDirectory, string(os.PathSeparator))
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), transID[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), transID[9:])
	return dir.String()
}

func (fsh *ObjectStorageHandler) DownloadFile(r *http.Request, allocationID string) (*DownloadResponse, *common.Error){
	if(r.Method == "POST") {
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

	filename,ok := r.URL.Query()["filename"]
	if !ok || len(filename[0]) < 1 {
        return nil, common.NewError("invalid_parameters", "path parameter not found")
    }
    fileName := filename[0]

	blobRefObject := allocation.getReferenceObject(filepath.Join(filePath,fileName), fileName, false, false)
	if blobRefObject == nil {
		Logger.Info("", zap.Any("blob_error", "Error getting the blob reference"))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return nil, common.NewError("invalid_parameters", "File not found. Please check the parameters")
	}

	if(blobRefObject.Header.ReferenceType != FILE ) {
		return nil, common.NewError("invalid_parameters", "Requested object is not a file. Please check the parameters")
	}
	
	blobRefObject.LoadReferenceEntries();

	part_num,ok := r.URL.Query()["part"]
	var partNum int
	if !ok || len(part_num[0]) < 1 {
        partNum = 1
        err = nil
    } else {
    	partNum, err = strconv.Atoi(part_num[0])
    } 
   

    if err!=nil || (partNum > len(blobRefObject.RefEntries) && partNum > 0) {
    	return nil, common.NewError("invalid_parameters", "invalid part number")
    }

	Logger.Info("", zap.Any("entries_size", len(blobRefObject.RefEntries)))

	// ReferenceType EntryType `csv:"type"`
	// Name string `csv:"name"`
	// LookupHash string `csv:"lookup_hash"`
	// PreviousRevisionHash string `csv:"previous_rev_hash"`
	// Size uint64 `csv:"size"`
	// IsCompressed bool `csv:"is_compressed"`
	partNum = partNum - 1
	response := &DownloadResponse{}
	response.Filename = blobRefObject.RefEntries[partNum].Name
	//response.Size = blobRefObject.RefEntries[0].Size
	dirPath, dirFileName := getFilePathFromHash(blobRefObject.RefEntries[partNum].LookupHash)
	response.Path = filepath.Join(allocation.ObjectsPath, dirPath, dirFileName)

	return response, nil
}


//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *ObjectStorageHandler) WriteFile(r *http.Request, allocationID string) (UploadResponse) {
	
	var response UploadResponse
	if r.Method == "GET" {
		return GenerateUploadResponseWithError(common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead")) 
	}

	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		//return -1, common.NewError("allocation_setup_error", err.Error())
		return GenerateUploadResponseWithError(common.NewError("allocation_setup_error", err.Error())) 
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {  
	   	Logger.Info("", zap.Any("error", err))
		//return -1, common.NewError("request_parse_error", err.Error())  
		return GenerateUploadResponseWithError(common.NewError("request_parse_error", err.Error())) 	
	}


	response.Result = make([]UploadResult, 0)
	for _, fheaders := range r.MultipartForm.File {
		var result UploadResult
		for _, hdr := range fheaders {
			blobObject, common_error := allocation.writeFileAndCalculateHash(&allocation.RootReferenceObject, hdr)
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


