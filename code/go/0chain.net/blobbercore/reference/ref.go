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
	ID             int64      `gorm:column:id;primary_key`
	Type           string     `gorm:"column:type" dirlist:"type" filelist:"type"`
	AllocationID   string     `gorm:"column:allocation_id"`
	Name           string     `gorm:"column:name" dirlist:"name" filelist:"name"`
	Path           string     `gorm:"column:path" dirlist:"path" filelist:"path"`
	Hash           string     `gorm:"column:hash" dirlist:"hash" filelist:"hash"`
	NumBlocks      int64      `gorm:"column:num_of_blocks" dirlist:"num_of_blocks" filelist:"num_of_blocks"`
	PathHash       string     `gorm:"column:path_hash" dirlist:"path_hash" filelist:"path_hash"`
	ParentPath     string     `gorm:"column:parent_path"`
	PathLevel      int        `gorm:"column:level"`
	CustomMeta     string     `gorm:"column:custom_meta" filelist:"custom_meta"`
	ContentHash    string     `gorm:"column:content_hash" filelist:"content_hash"`
	Size           int64      `gorm:"column:size" filelist:"size"`
	MerkleRoot     string     `gorm:"column:merkle_root" filelist:"merkle_root"`
	ActualFileSize int64      `gorm:"column:actual_file_size" filelist:"actual_file_size"`
	ActualFileHash string     `gorm:"column:actual_file_hash" filelist:"actual_file_hash"`
	MimeType       string     `gorm:"column:mimetype" filelist:"mimetype"`
	WriteMarker    string     `gorm:"column:write_marker"`
	Children       []*Ref     `gorm:"-"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
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

func GetReferenceFromPathHash(ctx context.Context, allocationID string, path_hash string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{AllocationID: allocationID, PathHash: path_hash}).First(ref).Error
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
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID}).Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})
	err := db.Order("level, name").Find(&refs).Error
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
	err := db.Order("level, name").Find(&refs).Error
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
	//fmt.Println("fileref name , path", fr.Name, fr.Path)
	//fmt.Println("Fileref hash : " + fr.GetHashData())
	fr.Hash = encryption.Hash(fr.GetFileHashData())
	fr.NumBlocks = int64(math.Ceil(float64(fr.Size*1.0) / CHUNK_SIZE))
	fr.PathHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	fr.PathLevel = len(GetSubDirsFromPath(fr.Path)) + 1 //strings.Count(fr.Path, "/")
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
	//fmt.Println("ref name and path " + r.Name + " " + r.Path)
	//fmt.Println("ref hash : " + strings.Join(childHashes, ":"))
	r.Hash = encryption.Hash(strings.Join(childHashes, ":"))
	r.NumBlocks = refNumBlocks
	//fmt.Println("Ref Path hash: " + strings.Join(childPathHashes, ":"))
	r.PathHash = encryption.Hash(strings.Join(childPathHashes, ":"))
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1 //strings.Count(r.Path, "/")

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
		return strings.Compare(r.Children[i].Name, r.Children[j].Name) == -1
	})
}

func (r *Ref) RemoveChild(idx int) {
	if idx < 0 {
		return
	}
	r.Children = append(r.Children[:idx], r.Children[idx+1:]...)
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].Name, r.Children[j].Name) == -1
	})
}

func DeleteReference(ctx context.Context, refID int64, pathHash string) error {
	if refID <= 0 {
		return common.NewError("invalid_ref_id", "Invalid reference ID to delete")
	}
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Where("path_hash = ?", pathHash).Delete(&Ref{ID: refID}).Error
}

// func (r *Ref) LoadChildren(ctx context.Context, dbStore datastore.Store) error {
// 	r.Children = make([]RefEntity, len(r.ChildRefs))
// 	if len(r.ChildRefs) > 0 {
// 		for index, childRef := range r.ChildRefs {
// 			var childRefObj datastore.Entity
// 			if strings.HasPrefix(childRef, fileRefEntityMetaData.GetDBName()) {
// 				childRefObj = fileRefEntityMetaData.Instance().(*FileRef)
// 			} else {
// 				childRefObj = refEntityMetaData.Instance().(*Ref)
// 			}

// 			err := dbStore.Read(ctx, childRef, childRefObj)
// 			if err != nil {
// 				return err
// 			}
// 			r.Children[index] = childRefObj.(RefEntity)
// 		}
// 	}
// 	return nil
// }

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

/*

func (r *Ref) CalculateHash(ctx context.Context, dbStore datastore.Store) (string, error) {
	err := r.LoadChildren(ctx, dbStore)
	if err != nil {
		return "", err
	}
	childHashes := make([]string, len(r.Children))
	childPathHashes := make([]string, len(r.Children))
	var refNumBlocks int64
	for index, childRef := range r.Children {
		childHashes[index] = childRef.GetHash(ctx)
		childPathHashes[index] = childRef.GetPathHash()
		refNumBlocks += childRef.GetNumBlocks(ctx)
	}
	//fmt.Println("ref hash : " + strings.Join(childHashes, ":"))
	r.Hash = encryption.Hash(strings.Join(childHashes, ":"))
	r.NumBlocks = refNumBlocks
	//fmt.Println("Ref Path hash: " + strings.Join(childPathHashes, ":"))
	r.PathHash = encryption.Hash(strings.Join(childPathHashes, ":"))
	return r.Hash, nil
}

func (r *Ref) DeleteChild(childKey string) {
	i := 0 // output index
	for _, x := range r.ChildRefs {
		if x != childKey {
			r.ChildRefs[i] = x
			i++
		}
	}
	r.ChildRefs = r.ChildRefs[:i]
}

func (r *Ref) GetHash(ctx context.Context) string {
	return r.Hash
}

func (r *Ref) GetListingData(ctx context.Context) map[string]interface{} {
	return GetListingFieldsMap(*r)
}
func (r *Ref) GetType() string {
	return r.Type
}

func (r *Ref) GetNumBlocks(ctx context.Context) int64 {
	return r.NumBlocks
}

func (r *Ref) GetPathHash() string {
	return r.PathHash
}

func (r *Ref) GetPath() string {
	return r.Path
}

func (r *Ref) GetName() string {
	return r.Name
}

*/

/*func CreateDirRefsIfNotExists(ctx context.Context, allocation string, path string, childKey string, dbStore datastore.Store) error {

	if len(path) == 0 {
		return common.NewError("invalid_parameters", "Invalid path to reference")
	}
	parentdir, currdir := filepath.Split(path)
	parentdir = filepath.Clean(parentdir)

	isRoot := false
	if len(currdir) == 0 {
		//Path is root
		isRoot = true
	}

	dirref, _ := RefProvider().(*Ref)
	dirref.AllocationID = allocation
	dirref.Name = currdir
	dirref.Path = path

	//dbStore := refEntityMetaData.GetStore()
	err := dbStore.Read(ctx, dirref.GetKey(), dirref)
	if err == datastore.ErrKeyNotFound {
		dirref.Type = DIRECTORY
		if !isRoot {
			parentDirRef, _ := RefProvider().(*Ref)
			parentDirRef.AllocationID = allocation
			parentDirRef.Path = parentdir
			parentDirRef.Type = DIRECTORY

			dirref.ParentRef = parentDirRef.GetKey()
		}
	}

	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}

	if dirref.Type != DIRECTORY {
		return common.NewError("invalid_reference", "Reference is not a directory. Path : "+dirref.Path)
	}

	if len(childKey) > 0 {
		existingChild := false
		for _, a := range dirref.ChildRefs {
			if a == childKey {
				existingChild = true
			}
		}
		if !existingChild {
			dirref.ChildRefs = append(dirref.ChildRefs, childKey)
		}

	}
	err = dbStore.Write(ctx, dirref)
	if err != nil {
		return err
	}

	if !isRoot {
		err = CreateDirRefsIfNotExists(ctx, allocation, parentdir, dirref.GetKey(), dbStore)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetRootReferenceFromStore(ctx context.Context, allocationID string, dbStore datastore.Store) (*Ref, error) {
	parentDirRef, _ := RefProvider().(*Ref)
	parentDirRef.AllocationID = allocationID
	parentDirRef.Path = "/"
	err := dbStore.Read(ctx, parentDirRef.GetKey(), parentDirRef)
	if err != nil {
		return nil, err
	}
	return parentDirRef, nil
}

func GetRootReference(ctx context.Context, allocationID string) (*Ref, error) {
	return GetRootReferenceFromStore(ctx, allocationID, refEntityMetaData.GetStore())
}

func RecalculateHashBottomUp(ctx context.Context, curRef *Ref, dbStore datastore.Store) error {

	parentdir, currdir := filepath.Split(curRef.Path)
	parentdir = filepath.Clean(parentdir)
	isRoot := false
	if len(currdir) == 0 {
		//Path is root
		isRoot = true
	}

	if curRef.Type != DIRECTORY {
		return common.NewError("invalid_reference", "Reference is not a directory. Path : "+curRef.Path)
	}

	_, err := curRef.CalculateHash(ctx, dbStore)
	if err != nil {
		return err
	}

	err = dbStore.Write(ctx, curRef)
	if err != nil {
		return err
	}

	if !isRoot {
		parentDirRef, _ := RefProvider().(*Ref)
		parentDirRef.AllocationID = curRef.AllocationID
		parentDirRef.Path = parentdir
		err = dbStore.Read(ctx, parentDirRef.GetKey(), parentDirRef)
		if err != nil {
			return err
		}
		err = RecalculateHashBottomUp(ctx, parentDirRef, dbStore)
		if err != nil {
			return err
		}
	}

	return nil
}*/

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
			curRef, err := GetRefWithSortedChildren(ctx, allocationID, child.Path)
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

	return &retObj, nil
}

/*
func GetObjectPathFromFilePath(ctx context.Context, allocationID string, filepath string, dbStore datastore.Store) (*ObjectPath, error) {

	rootRef, err := GetRootReferenceFromStore(ctx, allocationID, dbStore)
	//fmt.Println("Root ref found with hash : " + rootRef.Hash)
	if err != nil {
		return nil, common.NewError("invalid_dir_struct", "Allocation root corresponds to an invalid directory structure")
	}

	if len(filepath) == 0 {
		return nil, common.NewError("invalid_block_num", "Invalid filepath")
	}

	fileref := FileRefProvider().(*FileRef)
	fileref.AllocationID = allocationID
	fileref.Path = filepath
	err = fileref.Read(ctx, fileref.GetKey())
	if err != nil {
		return nil, common.NewError("invalid_block_num", "Invalid filepath. File not found")
	}

	curRef := RefProvider().(*Ref)
	err = dbStore.Read(ctx, fileref.ParentRef, curRef)
	if err != nil {
		return nil, common.NewError("parent_read_error", "Error reading the ref of the parent")
	}
	//result := //curRef.GetListingData(ctx)
	//curResult := result
	prevPath := ""
	var prevResult map[string]interface{}
	for true {
		newResult := curRef.GetListingData(ctx)
		err := curRef.LoadChildren(ctx, dbStore)
		if err != nil {
			return nil, common.NewError("error_loading_children", "Error loading children from store for path "+curRef.Path)
		}
		list := make([]map[string]interface{}, len(curRef.Children))
		for idx, child := range curRef.Children {
			if len(prevPath) > 0 && prevResult != nil {
				if child.GetPath() == prevPath {
					list[idx] = prevResult
					continue
				}
			}
			list[idx] = child.GetListingData(ctx)
		}
		newResult["list"] = list
		prevPath = curRef.GetPath()
		prevResult = newResult
		if curRef.GetPath() == rootRef.GetPath() {
			break
		}
		err = dbStore.Read(ctx, curRef.ParentRef, curRef)
		if err != nil {
			return nil, common.NewError("parent_read_error", "Error reading the ref of the parent")
		}
	}

	var retObj ObjectPath
	retObj.RootHash = rootRef.Hash
	retObj.Meta = fileref.GetListingData(ctx)
	retObj.Path = prevResult
	retObj.FileBlockNum = -1

	return &retObj, nil
}

*/
