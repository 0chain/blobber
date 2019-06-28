package reference

import (
	"context"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"0chain.net/blobbercore/datastore"
	"0chain.net/core/common"
	"0chain.net/core/encryption"

	"github.com/jinzhu/gorm"
)

const (
	FILE      = "f"
	DIRECTORY = "d"
)

const CHUNK_SIZE = 64 * 1024

const DIR_LIST_TAG = "dirlist"
const FILE_LIST_TAG = "filelist"

// type RefEntity interface {
// 	GetNumBlocks() int64
// 	GetHash() string
// 	CalculateHash() (string, error)
// 	GetListingData() map[string]interface{}
// 	GetType() string
// 	GetPathHash() string
// 	GetPath() string
// 	GetName() string
// }

type Ref struct {
	ID                  int64      `gorm:column:id;primary_key`
	Type                string     `gorm:"column:type" dirlist:"type" filelist:"type"`
	AllocationID        string     `gorm:"column:allocation_id"`
	LookupHash          string     `gorm:"column:lookup_hash" dirlist:"lookup_hash" filelist:"lookup_hash"`
	Name                string     `gorm:"column:name" dirlist:"name" filelist:"name"`
	Path                string     `gorm:"column:path" dirlist:"path" filelist:"path"`
	Hash                string     `gorm:"column:hash" dirlist:"hash" filelist:"hash"`
	NumBlocks           int64      `gorm:"column:num_of_blocks" dirlist:"num_of_blocks" filelist:"num_of_blocks"`
	PathHash            string     `gorm:"column:path_hash" dirlist:"path_hash" filelist:"path_hash"`
	ParentPath          string     `gorm:"column:parent_path"`
	PathLevel           int        `gorm:"column:level"`
	CustomMeta          string     `gorm:"column:custom_meta" filelist:"custom_meta"`
	ContentHash         string     `gorm:"column:content_hash" filelist:"content_hash"`
	Size                int64      `gorm:"column:size" filelist:"size"`
	MerkleRoot          string     `gorm:"column:merkle_root" filelist:"merkle_root"`
	ActualFileSize      int64      `gorm:"column:actual_file_size" filelist:"actual_file_size"`
	ActualFileHash      string     `gorm:"column:actual_file_hash" filelist:"actual_file_hash"`
	MimeType            string     `gorm:"column:mimetype" filelist:"mimetype"`
	WriteMarker         string     `gorm:"column:write_marker"`
	ThumbnailSize       int64      `gorm:"column:thumbnail_size" filelist:"thumbnail_size"`
	ThumbnailHash       string     `gorm:"column:thumbnail_hash" filelist:"thumbnail_hash"`
	ActualThumbnailSize int64      `gorm:"column:actual_thumbnail_size" filelist:"actual_thumbnail_size"`
	ActualThumbnailHash string     `gorm:"column:actual_thumbnail_hash" filelist:"actual_thumbnail_hash"`
	Children            []*Ref     `gorm:"-"`
	DeletedAt           *time.Time `gorm:"column:deleted_at"`
	datastore.ModelWithTS
}

func (Ref) TableName() string {
	return "reference_objects"
}

func GetReferenceLookup(allocationID string, path string) string {
	return encryption.Hash(allocationID + ":" + path)
}

func NewDirectoryRef() *Ref {
	return &Ref{Type: DIRECTORY}
}

func NewFileRef() *Ref {
	return &Ref{Type: FILE}
}

func GetReference(ctx context.Context, allocationID string, path string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{AllocationID: allocationID, Path: path}).First(ref).Error
	if err == nil {
		return ref, nil
	}
	return nil, err
}

func GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{AllocationID: allocationID, LookupHash: path_hash}).First(ref).Error
	if err == nil {
		return ref, nil
	}
	return nil, err
}

func GetSubDirsFromPath(p string) []string {
	path := p
	parent, cur := filepath.Split(path)
	parent = filepath.Clean(parent)
	subDirs := make([]string, 0)
	for len(cur) > 0 {
		subDirs = append([]string{cur}, subDirs...)
		parent, cur = filepath.Split(parent)
		parent = filepath.Clean(parent)
	}
	return subDirs
}

func GetRefWithChildren(ctx context.Context, allocationID string, path string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID}).Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})
	err := db.Order("level, created_at").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID}, nil
	}
	curRef := &refs[0]
	if curRef.Path != path {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}
	for i := 1; i < len(refs); i++ {
		if refs[i].ParentPath == curRef.Path {
			curRef.Children = append(curRef.Children, &refs[i])
		} else {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
	}
	return &refs[0], nil
}

func GetRefWithSortedChildren(ctx context.Context, allocationID string, path string) (*Ref, error) {
	var refs []*Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID}).Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})
	err := db.Order("level, lookup_hash").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID}, nil
	}
	curRef := refs[0]
	if curRef.Path != path {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}
	for i := 1; i < len(refs); i++ {
		if refs[i].ParentPath == curRef.Path {
			curRef.Children = append(curRef.Children, refs[i])
		} else {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
	}
	return refs[0], nil
}

func GetReferencePath(ctx context.Context, allocationID string, path string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID})
	depth := len(GetSubDirsFromPath(path)) + 1
	curPath := filepath.Dir(path)
	for i := 0; i < depth-1; i++ {
		db = db.Or(Ref{ParentPath: curPath, AllocationID: allocationID})
		curPath = filepath.Dir(curPath)
	}
	db = db.Or("parent_path = ? AND allocation_id = ?", "", allocationID)
	err := db.Order("level, lookup_hash").Find(&refs).Error
	if (err != nil && gorm.IsRecordNotFoundError(err)) || len(refs) == 0 {
		return &Ref{Type: DIRECTORY, AllocationID: allocationID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}, nil
	}
	if err != nil {
		return nil, err
	}

	curRef := &refs[0]
	if curRef.Path != "/" {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}
	curLevel := 2
	subDirs := GetSubDirsFromPath(path)
	var foundRef *Ref
	for i := 1; i < len(refs); i++ {
		if refs[i].ParentPath != curRef.Path && foundRef != nil {
			curLevel++
			curRef = foundRef
			foundRef = nil
		}

		if refs[i].ParentPath == curRef.Path {
			if subDirs[curLevel-2] == refs[i].Name {
				//curRef = &refs[i]
				foundRef = &refs[i]
			}
			curRef.Children = append(curRef.Children, &refs[i])
		} else {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
	}
	//refs[0].CalculateHash(ctx, false)
	return &refs[0], nil
}

func (fr *Ref) GetFileHashData() string {
	hashArray := make([]string, 0)
	hashArray = append(hashArray, fr.AllocationID)
	hashArray = append(hashArray, fr.Type)
	hashArray = append(hashArray, fr.Name)
	hashArray = append(hashArray, fr.Path)
	hashArray = append(hashArray, strconv.FormatInt(fr.Size, 10))
	hashArray = append(hashArray, fr.ContentHash)
	hashArray = append(hashArray, fr.MerkleRoot)
	hashArray = append(hashArray, strconv.FormatInt(fr.ActualFileSize, 10))
	hashArray = append(hashArray, fr.ActualFileHash)
	return strings.Join(hashArray, ":")
}

func (fr *Ref) CalculateFileHash(ctx context.Context, saveToDB bool) (string, error) {
	//fmt.Println("fileref name , path, hash", fr.Name, fr.Path, fr.Hash)
	//fmt.Println("Fileref hash data: " + fr.GetFileHashData())
	fr.Hash = encryption.Hash(fr.GetFileHashData())
	//fmt.Println("Fileref hash : " + fr.Hash)
	fr.NumBlocks = int64(math.Ceil(float64(fr.Size*1.0) / CHUNK_SIZE))
	fr.PathHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	fr.PathLevel = len(GetSubDirsFromPath(fr.Path)) + 1 //strings.Count(fr.Path, "/")
	fr.LookupHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	var err error
	if saveToDB {
		err = fr.Save(ctx)
	}
	return fr.Hash, err
}

func (r *Ref) CalculateDirHash(ctx context.Context, saveToDB bool) (string, error) {
	if len(r.Children) == 0 {
		return r.Hash, nil
	}
	for _, childRef := range r.Children {
		_, err := childRef.CalculateHash(ctx, saveToDB)
		if err != nil {
			return "", err
		}
	}
	childHashes := make([]string, len(r.Children))
	childPathHashes := make([]string, len(r.Children))
	var refNumBlocks int64
	for index, childRef := range r.Children {
		childHashes[index] = childRef.Hash
		childPathHashes[index] = childRef.PathHash
		refNumBlocks += childRef.NumBlocks
	}
	//fmt.Println("ref name and path, hash :" + r.Name + " " + r.Path + " " + r.Hash)
	//fmt.Println("ref hash data: " + strings.Join(childHashes, ":"))
	r.Hash = encryption.Hash(strings.Join(childHashes, ":"))
	//fmt.Println("ref hash : " + r.Hash)
	r.NumBlocks = refNumBlocks
	//fmt.Println("Ref Path hash: " + strings.Join(childPathHashes, ":"))
	r.PathHash = encryption.Hash(strings.Join(childPathHashes, ":"))
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1 //strings.Count(r.Path, "/")
	r.LookupHash = GetReferenceLookup(r.AllocationID, r.Path)
	var err error
	if saveToDB {
		err = r.Save(ctx)
	}

	return r.Hash, err
}

func (r *Ref) CalculateHash(ctx context.Context, saveToDB bool) (string, error) {
	if r.Type == DIRECTORY {
		return r.CalculateDirHash(ctx, saveToDB)
	}
	return r.CalculateFileHash(ctx, saveToDB)
}

func (r *Ref) AddChild(child *Ref) {
	if r.Children == nil {
		r.Children = make([]*Ref, 0)
	}
	r.Children = append(r.Children, child)
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].LookupHash, r.Children[j].LookupHash) == -1
	})
}

func (r *Ref) RemoveChild(idx int) {
	if idx < 0 {
		return
	}
	r.Children = append(r.Children[:idx], r.Children[idx+1:]...)
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].LookupHash, r.Children[j].LookupHash) == -1
	})
}

func DeleteReference(ctx context.Context, refID int64, pathHash string) error {
	if refID <= 0 {
		return common.NewError("invalid_ref_id", "Invalid reference ID to delete")
	}
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Where("path_hash = ?", pathHash).Delete(&Ref{ID: refID}).Error
}

func (r *Ref) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Save(r).Error
}

func (r *Ref) GetListingData(ctx context.Context) map[string]interface{} {
	if r.Type == FILE {
		return GetListingFieldsMap(*r, FILE_LIST_TAG)
	}
	return GetListingFieldsMap(*r, DIR_LIST_TAG)
}

func GetListingFieldsMap(refEntity interface{}, tagName string) map[string]interface{} {
	result := make(map[string]interface{})
	t := reflect.TypeOf(refEntity)
	v := reflect.ValueOf(refEntity)
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get(tagName)
		// Skip if tag is not defined or ignored
		if !field.Anonymous && (tag == "" || tag == "-") {
			continue
		}

		if field.Anonymous {
			listMap := GetListingFieldsMap(v.FieldByName(field.Name).Interface(), tagName)
			if len(listMap) > 0 {
				for k, v := range listMap {
					result[k] = v
				}

			}
		} else {
			fieldValue := v.FieldByName(field.Name).Interface()
			if fieldValue == nil {
				continue
			}
			result[tag] = fieldValue
		}

	}
	return result
}

type ObjectPath struct {
	RootHash     string                 `json:"root_hash"`
	Meta         map[string]interface{} `json:"meta_data"`
	Path         map[string]interface{} `json:"path"`
	FileBlockNum int64                  `json:"file_block_num"`
	RefID        int64                  `json:"-"`
}

func GetObjectPath(ctx context.Context, allocationID string, blockNum int64) (*ObjectPath, error) {

	rootRef, err := GetRefWithSortedChildren(ctx, allocationID, "/")
	//fmt.Println("Root ref found with hash : " + rootRef.Hash)
	if err != nil {
		return nil, common.NewError("invalid_dir_struct", "Allocation root corresponds to an invalid directory structure")
	}

	if rootRef.NumBlocks < blockNum {
		return nil, common.NewError("invalid_block_num", "Invalid block number"+string(rootRef.NumBlocks)+" / "+string(blockNum))
	}

	if rootRef.NumBlocks == 0 {
		var retObj ObjectPath
		retObj.RootHash = rootRef.Hash
		retObj.FileBlockNum = 0
		result := rootRef.GetListingData(ctx)
		list := make([]map[string]interface{}, len(rootRef.Children))
		for idx, child := range rootRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		result["list"] = list
		retObj.Path = result
		return &retObj, nil
	}

	found := false
	var curRef *Ref
	curRef = rootRef
	remainingBlocks := blockNum

	result := curRef.GetListingData(ctx)
	curResult := result

	for !found {
		list := make([]map[string]interface{}, len(curRef.Children))
		for idx, child := range curRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		curResult["list"] = list
		for idx, child := range curRef.Children {
			//result.Entities[idx] = child.GetListingData(ctx)

			if child.NumBlocks < remainingBlocks {
				remainingBlocks = remainingBlocks - child.NumBlocks
				continue
			}
			if child.Type == FILE {
				found = true
				curRef = child
				break
			}
			curRef, err = GetRefWithSortedChildren(ctx, allocationID, child.Path)
			if err != nil || len(curRef.Hash) == 0 {
				return nil, common.NewError("failed_object_path", "Failed to get the object path")
			}
			curResult = list[idx]
			break
		}
	}
	if !found {
		return nil, common.NewError("invalid_parameters", "Block num was not found")
	}

	var retObj ObjectPath
	retObj.RootHash = rootRef.Hash
	retObj.Meta = curRef.GetListingData(ctx)
	retObj.Path = result
	retObj.FileBlockNum = remainingBlocks
	retObj.RefID = curRef.ID
	//rootRef.CalculateHash(ctx, false)
	return &retObj, nil
}
