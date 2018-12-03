package directoryref

import (
	"context"
	"path/filepath"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/reference"
)

func CreateDirRefsIfNotExists(ctx context.Context, allocation string, path string, child *reference.Ref) error {
	if len(path) == 0 {
		return nil
	}
	parentdir, currdir := filepath.Split(path)

	dirref, _ := Provider().(*DirectoryRef)
	dirref.AllocationID = allocation
	dirref.Name = currdir
	dirref.Path = path

	dbStore := dirRefEntityMetaData.GetStore()
	err := dbStore.Read(ctx, dirref.GetKey(), dirref)
	if err == datastore.ErrKeyNotFound {
		dirref.Type = reference.DIRECTORY

		parentDirRef, _ := Provider().(*DirectoryRef)
		parentDirRef.AllocationID = allocation
		parentDirRef.Path = parentdir
		parentDirRef.Type = reference.DIRECTORY

		dirref.ParentRef = parentDirRef.GetKey()
	}

	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}

	if dirref.Type != reference.DIRECTORY {
		return common.NewError("invalid_reference", "Reference is not a directory. Path : "+dirref.Path)
	}

	if child != nil {
		dirref.ChildRefs = append(dirref.ChildRefs, child.GetKey())
	}
	err = dirref.Write(ctx)
	if err != nil {
		return err
	}

	err = CreateDirRefsIfNotExists(ctx, allocation, parentdir, dirref.Ref)

	return nil
}

type DirectoryRef struct {
	*reference.Ref
	Children []*reference.Ref
}

var dirRefEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &DirectoryRef{}
	t.Version = "1.0"
	t.CreationDate = common.Now()
	return t
}

func SetupEntity(store datastore.Store) {
	dirRefEntityMetaData = datastore.MetadataProvider()
	dirRefEntityMetaData.Name = "dirref"
	dirRefEntityMetaData.Provider = Provider
	dirRefEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("dirref", dirRefEntityMetaData)
}

func (dr *DirectoryRef) GetEntityMetadata() datastore.EntityMetadata {
	return dirRefEntityMetaData
}
func (dr *DirectoryRef) SetKey(key datastore.Key) {

}
func (dr *DirectoryRef) GetKey() datastore.Key {
	return datastore.ToKey("ref" + ":" + encryption.Hash(dr.AllocationID+dr.Path))
}
func (dr *DirectoryRef) Read(ctx context.Context, key datastore.Key) error {
	return dirRefEntityMetaData.GetStore().Read(ctx, key, dr)
}
func (dr *DirectoryRef) Write(ctx context.Context) error {
	return dirRefEntityMetaData.GetStore().Write(ctx, dr)
}
func (dr *DirectoryRef) Delete(ctx context.Context) error {
	return nil
}
