package reference

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
)

const (
	FILE      = "f"
	DIRECTORY = "d"
)

const LIST_TAG = "list"

type RefEntity interface {
	GetNumBlocks(context.Context) int64
	GetHash(context.Context) string
	CalculateHash(context.Context, datastore.Store) (string, error)
	GetListingData(context.Context) map[string]interface{}
	GetType() string
	GetPathHash() string
	GetPath() string
	GetName() string
}

type Ref struct {
	Version      string           `json:"version"`
	CreationDate common.Timestamp `json:"creation_date" list:"creation_date"`
	Type         string           `json:"type" list:"type"`
	AllocationID string           `json:"allocation_id"`
	Name         string           `json:"name" list:"name"`
	Path         string           `json:"path" list:"path"`
	Hash         string           `json:"hash" list:"hash"`
	NumBlocks    int64            `json:"num_of_blocks" list:"num_of_blocks"`
	PathHash     string           `json:"path_hash" list:"path_hash"`
	ParentRef    string           `json:"parent"`
	ChildRefs    []string         `json:"children"`
	Children     []RefEntity      `json:"-"`
}

var refEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func RefProvider() datastore.Entity {
	t := &Ref{}
	t.Version = "1.0"
	t.CreationDate = common.Now()
	t.Type = DIRECTORY
	return t
}

func SetupRefEntity(store datastore.Store) {
	refEntityMetaData = datastore.MetadataProvider()
	refEntityMetaData.Name = "ref"
	refEntityMetaData.DB = "ref"
	refEntityMetaData.Provider = RefProvider
	refEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("ref", refEntityMetaData)
}

func (r *Ref) GetEntityMetadata() datastore.EntityMetadata {
	return refEntityMetaData
}

func (r *Ref) GetKey() string {
	return "ref:" + GetReferenceLookup(r.AllocationID, r.Path)
}
func (r *Ref) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}
func (r *Ref) Read(ctx context.Context, key datastore.Key) error {
	return refEntityMetaData.GetStore().Read(ctx, key, r)
}
func (r *Ref) Write(ctx context.Context) error {
	return refEntityMetaData.GetStore().Write(ctx, r)
}
func (r *Ref) Delete(ctx context.Context) error {
	return nil
}

func GetReferenceLookup(allocationID string, path string) string {
	return encryption.Hash(allocationID + ":" + path)
}

func (r *Ref) LoadChildren(ctx context.Context, dbStore datastore.Store) error {
	r.Children = make([]RefEntity, len(r.ChildRefs))
	if len(r.ChildRefs) > 0 {
		for index, childRef := range r.ChildRefs {
			var childRefObj datastore.Entity
			if strings.HasPrefix(childRef, fileRefEntityMetaData.GetDBName()) {
				childRefObj = fileRefEntityMetaData.Instance().(*FileRef)
			} else {
				childRefObj = refEntityMetaData.Instance().(*Ref)
			}

			err := dbStore.Read(ctx, childRef, childRefObj)
			if err != nil {
				return err
			}
			r.Children[index] = childRefObj.(RefEntity)
		}
	}
	return nil
}

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

func GetListingFieldsMap(refEntity interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	t := reflect.TypeOf(refEntity)
	v := reflect.ValueOf(refEntity)
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get(LIST_TAG)
		// Skip if tag is not defined or ignored
		if !field.Anonymous && (tag == "" || tag == "-") {
			continue
		}

		if field.Anonymous {
			listMap := GetListingFieldsMap(v.FieldByName(field.Name).Interface())
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

func CreateDirRefsIfNotExists(ctx context.Context, allocation string, path string, childKey string, dbStore datastore.Store) error {

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
}

type ObjectPath struct {
	RootHash     string                 `json:"root_hash"`
	Meta         map[string]interface{} `json:"meta_data"`
	Path         map[string]interface{} `json:"path"`
	FileBlockNum int64                  `json:"file_block_num"`
}

func GetObjectPath(ctx context.Context, allocationID string, blockNum int64, dbStore datastore.Store) (*ObjectPath, error) {

	rootRef, err := GetRootReferenceFromStore(ctx, allocationID, dbStore)
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
		err := rootRef.LoadChildren(ctx, dbStore)
		if err != nil {
			return nil, common.NewError("error_loading_children", "Error loading children from store for root path ")
		}
		list := make([]map[string]interface{}, len(rootRef.Children))
		for idx, child := range rootRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		result["list"] = list
		retObj.Path = result
		return &retObj, nil
	}

	found := false
	var curRef RefEntity
	curRef = rootRef
	remainingBlocks := blockNum

	result := curRef.GetListingData(ctx)
	curResult := result

	for !found {
		err := curRef.(*Ref).LoadChildren(ctx, dbStore)
		if err != nil {
			return nil, common.NewError("error_loading_children", "Error loading children from store for path "+curRef.(*Ref).Path)
		}
		list := make([]map[string]interface{}, len(curRef.(*Ref).Children))
		for idx, child := range curRef.(*Ref).Children {
			list[idx] = child.GetListingData(ctx)
		}
		curResult["list"] = list
		for idx, child := range curRef.(*Ref).Children {
			//result.Entities[idx] = child.GetListingData(ctx)

			if child.GetNumBlocks(ctx) < remainingBlocks {
				remainingBlocks = remainingBlocks - child.GetNumBlocks(ctx)
				continue
			}
			if child.GetType() == FILE {
				found = true
				curRef = child
				break
			}
			curRef = child
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

	return &retObj, nil
}

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
