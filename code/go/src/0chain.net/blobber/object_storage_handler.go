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
)

//ObjectStorageHandler - implments the StorageHandler interface
type ObjectStorageHandler struct {
	RootDirectory string
}

const (
	OSPathSeperator string = string(os.PathSeparator)
	RefsDirName = "refs"
	ObjectsDirName = "objects"
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
	ReferenceType EntryType
	Name string
	LookupHash string
	PreviousRevisionHash string
	Size uint64
	IsCompressed bool	
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
	RefEntries []*ReferenceEntry 
}

type BlobObject struct {
	ID string
	Hash string
	FilenameHash string
	Path string
	ActualPath string
	Filename string
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

func (allocation *Allocation) newRootReferenceObject() *ReferenceObject {

	refObject := &ReferenceObject{ActualPath: OSPathSeperator, ActualFilename : OSPathSeperator}
	refObject.Hash = util.Hash(refObject.ActualPath)
	refObject.ID = fmt.Sprintf("%s%s%s", allocation.ID, "-", refObject.Hash)
	path, filename := getFilePathFromHash(refObject.Hash);
	refObject.Path = filepath.Join(allocation.RefsPath, path)
	refObject.Filename = filename
	refObject.FullPath = filepath.Join(refObject.Path, refObject.Filename)
	return refObject
}

func (allocation *Allocation) writeFileAndCalculateHash(parentRef *ReferenceObject, fileHeader *multipart.FileHeader) (*BlobObject, *common.Error){
	blobObject := &BlobObject{Filename : fileHeader.Filename, ActualPath: filepath.Join(parentRef.ActualPath, fileHeader.Filename)}
	h := sha1.New()
	tempFilePath := filepath.Join(allocation.TempObjectsPath, blobObject.Filename + "." + encryption.Hash(blobObject.ActualPath))
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
	allocation.TempObjectsPath = filepath.Join(allocation.ObjectsPath, "tmp")
	
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

	root_ref := allocation.newRootReferenceObject()

	//create the root reference dirs
	err = util.CreateDirs(root_ref.Path)
	if err != nil {
		Logger.Info("allocation_root_dir_creation_error", zap.Any("allocation_root_dir_creation_error", err))
		return nil, err
	}

	if _, err := os.Stat(root_ref.FullPath); err != nil {
		var fh *os.File;
	    if os.IsNotExist(err) {
	        //create the root reference file
			fh, err = os.Create(root_ref.FullPath)
			if err != nil {
				Logger.Info("allocation_root_file_creation_error", zap.Any("allocation_root_file_creation_error", err))
				return nil, err
			}
			defer fh.Close()
			root_ref.Header.Version = "1.0"
			root_ref.Header.ReferenceType = DIRECTORY
			w := bufio.NewWriter(fh)
			w.WriteString(strings.Join(root_ref.GetHeaders(), ",") + "\n")
			w.Flush()
	    }
	} else {
    	fh, err := os.Open(root_ref.FullPath)
		if err != nil {
			Logger.Info("allocation_root_file_open_error", zap.Any("allocation_root_file_open_error", err))
			return nil, err
		}
		defer fh.Close()
		r:= bufio.NewReader(fh)
		header,_ := r.ReadString('\n')
		header = strings.TrimSuffix(header, "\n")
		Logger.Info("", zap.Any("header", header))
		root_ref.LoadHeader(strings.Split(header, ","))

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


//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *ObjectStorageHandler) WriteFile(r *http.Request, allocationID string) (int64, *common.Error) {
	
	if r.Method == "GET" {
		return -1, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead")
	}

	allocation, err := fsh.setupAllocation(allocationID)

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		return -1, common.NewError("allocation_setup_error", err.Error())
	}

	// parse request  
	const MAX_MEMORY = 10 * 1024 * 1024 
	if err = r.ParseMultipartForm(MAX_MEMORY); nil != err {  
	   	Logger.Info("", zap.Any("error", err))
		return -1, common.NewError("request_parse_error", err.Error())  
	}

	for _, fheaders := range r.MultipartForm.File {
		for _, hdr := range fheaders {
			_, common_error := allocation.writeFileAndCalculateHash(&allocation.RootReferenceObject, hdr)
			if common_error != nil {
				return -1, common_error  		
			}
		}
	}

	return 10, nil
}


