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
	GetHash(context.Context) string
	CalculateHash(context.Context) (string, error)
	GetListingData(context.Context) map[string]interface{}
	GetType() string
}

type Ref struct {
	Version      string           `json:"version"`
	CreationDate common.Timestamp `json:"creation_date" list:"creation_date"`
	Type         string           `json:"type" list:"type"`
	AllocationID string           `json:"allocation_id"`
	Name         string           `json:"name" list:"name"`
	Path         string           `json:"path" list:"path"`
	Hash         string           `json:"hash" list:"hash"`
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

func (r *Ref) LoadChildren(ctx context.Context) error {
	r.Children = make([]RefEntity, len(r.ChildRefs))
	if len(r.ChildRefs) > 0 {
		dbStore := refEntityMetaData.GetStore()
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

func (r *Ref) CalculateHash(ctx context.Context) (string, error) {
	err := r.LoadChildren(ctx)
	if err != nil {
		return "", err
	}
	childHashes := make([]string, len(r.Children))
	for index, childRef := range r.Children {
		childHashes[index] = childRef.GetHash(ctx)
	}
	r.Hash = encryption.Hash(strings.Join(childHashes, ":"))
	return r.Hash, nil
}

func (r *Ref) GetHash(context.Context) string {
	return r.Hash
}

func (r *Ref) GetListingData(context.Context) map[string]interface{} {
	return GetListingFieldsMap(*r)
}
func (r *Ref) GetType() string {
	return r.Type
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
			result[tag] = v.FieldByName(field.Name).Interface()
		}

	}
	return result
}

func CreateDirRefsIfNotExists(ctx context.Context, allocation string, path string, childKey string) error {

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

	dbStore := refEntityMetaData.GetStore()
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
		dirref.ChildRefs = append(dirref.ChildRefs, childKey)
	}
	err = dirref.Write(ctx)
	if err != nil {
		return err
	}

	if !isRoot {
		err = CreateDirRefsIfNotExists(ctx, allocation, parentdir, dirref.GetKey())
		if err != nil {
			return err
		}
	}

	return nil
}

func GetRootReference(ctx context.Context, allocationID string) (*Ref, error) {
	parentDirRef, _ := RefProvider().(*Ref)
	parentDirRef.AllocationID = allocationID
	parentDirRef.Path = "/"
	err := parentDirRef.Read(ctx, parentDirRef.GetKey())
	if err != nil {
		return nil, err
	}
	return parentDirRef, nil
}

func RecalculateHashBottomUp(ctx context.Context, curRef *Ref) error {

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

	_, err := curRef.CalculateHash(ctx)
	if err != nil {
		return err
	}
	err = curRef.Write(ctx)
	if err != nil {
		return err
	}

	if !isRoot {
		parentDirRef, _ := RefProvider().(*Ref)
		parentDirRef.AllocationID = curRef.AllocationID
		parentDirRef.Path = parentdir
		err = parentDirRef.Read(ctx, parentDirRef.GetKey())
		if err != nil {
			return err
		}
		err = RecalculateHashBottomUp(ctx, parentDirRef)
		if err != nil {
			return err
		}
	}

	return nil
}
