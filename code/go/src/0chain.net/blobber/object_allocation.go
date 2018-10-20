package blobber

import (
	"0chain.net/common"
	"0chain.net/encryption"
	. "0chain.net/logging"
	"0chain.net/util"
	"0chain.net/writemarker"
	"golang.org/x/crypto/sha3"

	"path/filepath"

	"go.uber.org/zap"

	"bufio"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"io"
	"mime/multipart"
	"strings"

	"github.com/jszwec/csvutil"

	"os"

	"bytes"
	"fmt"
)

type Allocation struct {
	ID                  string
	Path                string
	ObjectsPath         string
	TempObjectsPath     string
	RefsPath            string
	RootReferenceObject ReferenceObject
}

type ReferenceHeader struct {
	Version       string
	ReferenceType EntryType
}

type ReferenceEntry struct {
	ReferenceType        EntryType `csv:"type"`
	Name                 string    `csv:"name"`
	LookupHash           string    `csv:"lookup_hash"`
	PreviousRevisionHash string    `csv:"previous_rev_hash"`
	Size                 int64     `csv:"size"`
	IsCompressed         bool      `csv:"is_compressed"`
	CustomMeta           string    `csv:"custom_meta"`
}

type ReferenceObject struct {
	ID             string
	Hash           string
	Path           string
	Filename       string
	FullPath       string
	ActualPath     string
	ActualFilename string
	Header         ReferenceHeader
	RefEntries     []ReferenceEntry
}

type BlobObject struct {
	ID           string
	Hash         string
	FilenameHash string
	Path         string
	ActualPath   string
	Filename     string
	Ref          *ReferenceObject
}

type EntryType int

const (
	FILE EntryType = 1 + iota
	DIRECTORY
)

func (e EntryType) String() string {
	switch e {
	case FILE:
		return "f"
	case DIRECTORY:
		return "d"
	}
	return ""
}

func ParseEntryType(s string) EntryType {
	if s == "f" {
		return FILE
	} else if s == "d" {
		return DIRECTORY
	}
	return -1
}

func getFilePathFromHash(hash string) (string, string) {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s", hash[0:3])
	for i := 1; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), hash[3*i:3*i+3])
	}
	return dir.String(), hash[9:]
}

func (allocation *Allocation) getReferenceObject(relativePath string, filename string, isDir bool, shouldCreateRef bool) (*ReferenceObject, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	isNew := false
	refObject := &ReferenceObject{ActualPath: relativePath, ActualFilename: filename}
	refObject.Hash = util.Hash(refObject.ActualPath)
	refObject.ID = fmt.Sprintf("%s%s%s", allocation.ID, "-", refObject.Hash)
	path, filename := getFilePathFromHash(refObject.Hash)
	refObject.Path = filepath.Join(allocation.RefsPath, path)
	refObject.Filename = filename
	refObject.FullPath = filepath.Join(refObject.Path, refObject.Filename)

	//create the root reference dirs
	err := util.CreateDirs(refObject.Path)
	if err != nil {
		Logger.Info("reference_dir_creation_error", zap.Any("reference_dir_creation_error", err))
		return nil, isNew
	}

	if _, err := os.Stat(refObject.FullPath); err != nil {
		var fh *os.File
		if os.IsNotExist(err) {
			if !shouldCreateRef {
				return nil, isNew
			}
			//create the root reference file
			fh, err = os.Create(refObject.FullPath)
			if err != nil {
				Logger.Info("reference_file_creation_error", zap.Any("reference_file_creation_error", err))
				return nil, isNew
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
			isNew = true
		}
	} else {
		fh, err := os.Open(refObject.FullPath)
		if err != nil {
			Logger.Info("reference_file_open_error", zap.Any("reference_file_open_error", err))
			return nil, isNew
		}
		defer fh.Close()
		r := bufio.NewReader(fh)
		header, _ := r.ReadString('\n')
		header = strings.TrimSuffix(header, "\n")
		refObject.LoadHeader(strings.Split(header, ","))
		isNew = false
	}

	return refObject, isNew
}

func (allocation *Allocation) writeFileAndCalculateHash(parentRef *ReferenceObject, fileHeader *multipart.FileHeader, customMeta string, wm *writemarker.WriteMarker) (*BlobObject, *common.Error) {
	blobRefObject, isNew := allocation.getReferenceObject(filepath.Join(parentRef.ActualPath, fileHeader.Filename), fileHeader.Filename, false, true)
	mutex.Lock()
	defer mutex.Unlock()
	blobObject := &BlobObject{Filename: fileHeader.Filename, Ref: blobRefObject}

	h := sha1.New()
	tempFilePath := filepath.Join(allocation.TempObjectsPath, blobObject.Filename+"."+encryption.Hash(blobObject.Ref.ActualPath)+"."+encryption.Hash(string(common.Now())))
	dest, err := os.Create(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()
	infile, err := fileHeader.Open()
	if err != nil {
		return nil, common.NewError("file_reading_error", err.Error())
	}
	merkleHash := sha3.New256()
	multiHashWriter := io.MultiWriter(h, merkleHash)
	tReader := io.TeeReader(infile, multiHashWriter)
	merkleLeaves := make([]util.Hashable, 0)
	for true {
		_, err := io.CopyN(dest, tReader, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}
		merkleLeaves = append(merkleLeaves, util.NewStringHashable(hex.EncodeToString(merkleHash.Sum(nil))))
		merkleHash.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}

	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)
	//Logger.Info("Calculated Merkle root", zap.String("merkle_root", mt.GetRoot()), zap.Int("merkle_leaf_count", len(merkleLeaves)))

	blobObject.Hash = hex.EncodeToString(h.Sum(nil))

	//move file from tmp location to the objects folder
	dirPath, destFile := getFilePathFromHash(blobObject.Hash)
	blobObject.Path = filepath.Join(allocation.ObjectsPath, dirPath)
	err = util.CreateDirs(blobObject.Path)
	if err != nil {
		return nil, common.NewError("blob_object_dir_creation_error", err.Error())
	}
	blobObject.Path = filepath.Join(blobObject.Path, destFile)
	err = os.Rename(tempFilePath, blobObject.Path)

	if err != nil {
		return nil, common.NewError("blob_object_creation_error", err.Error())
	}

	if parentRef.Header.ReferenceType == DIRECTORY && isNew {
		err = parentRef.AppendReferenceEntry(&ReferenceEntry{ReferenceType: FILE, Name: blobObject.Filename, LookupHash: blobRefObject.Hash})
		if err != nil {
			return nil, common.NewError("reference_parent_append_error", err.Error())
		}
	}

	err = blobRefObject.AppendReferenceEntry(&ReferenceEntry{ReferenceType: FILE, Name: blobObject.Filename, LookupHash: blobObject.Hash, Size: fileHeader.Size, CustomMeta: customMeta})
	if err != nil {
		return nil, common.NewError("reference_object_append_error", err.Error())
	}

	wmentity := writemarker.Provider()
	writeMarker, _ := wmentity.(*writemarker.WriteMarkerEntity)
	writeMarker.AllocationID = allocation.ID
	writeMarker.ContentHash = blobObject.Hash
	writeMarker.MerkleRoot = mt.GetRoot()
	writeMarker.WM = wm
	writeMarker.ContentSize = fileHeader.Size
	writeMarker.Status = writemarker.Accepted
	err = writeMarker.Write(common.GetRootContext())
	if err != nil {
		return nil, common.NewError("error_persisting_write_marker", err.Error())
	}
	go GetProtocolImpl(allocation.ID).RedeemMarker(wmentity.(*writemarker.WriteMarkerEntity))
	// debugEntity := writemarker.Provider()
	// badgerdbstore.GetStorageProvider().Read(common.GetRootContext(), writeMarker.GetKey(), debugEntity)
	// Logger.Info("Debugging to see if saving was successful", zap.Any("wm", debugEntity))

	return blobObject, nil
}

func (refObject *ReferenceObject) AppendReferenceEntry(entry *ReferenceEntry) error {

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

func (refObject *ReferenceObject) LoadReferenceEntries() error {
	fh, err := os.Open(refObject.FullPath)
	if err != nil {
		Logger.Info("reference_object_open_error", zap.Any("reference_object_open_error", err))
		return err
	}
	defer fh.Close()

	r := bufio.NewReader(fh)
	//read the first line and ignore since it the header
	r.ReadString('\n')

	csvReader := csv.NewReader(r)

	dec, err := csvutil.NewDecoder(csvReader, "type", "name", "lookup_hash", "previous_rev_hash", "size", "is_compressed", "custom_meta")
	if err != nil {
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

	return nil
}

func (refObject *ReferenceObject) GetHeaders() []string {
	return []string{refObject.Header.Version, refObject.Header.ReferenceType.String()}
}

func (refObject *ReferenceObject) LoadHeader(headers []string) {
	if len(headers) > 1 {
		refObject.Header.Version = headers[0]
		refObject.Header.ReferenceType = ParseEntryType(headers[1])
	}
}
