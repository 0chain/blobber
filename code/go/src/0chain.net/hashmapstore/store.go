package hashmapstore

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"0chain.net/datastore"
)

var storageAPI *Store

/*SetupStorageProvider - sets up hashmapstore and opens */
func SetupStorageProvider() {
	storageAPI = NewStore()
}

/*GetStorageProvider - get the storage provider for the hashmapstore */
func GetStorageProvider() datastore.Store {
	return storageAPI
}

func NewStore() *Store {
	retStore := &Store{}
	retStore.DB = make(map[datastore.Key][]byte)
	return retStore
}

/*Store - just a struct to implement the datastore.Store interface */
type Store struct {
	DB map[datastore.Key][]byte
}

/*Read - read an entity from the store */
func (ps *Store) Read(ctx context.Context, key datastore.Key, entity datastore.Entity) error {
	item, ok := ps.DB[key]

	if !ok {
		return datastore.ErrKeyNotFound
	}
	err := json.NewDecoder(bytes.NewReader(item)).Decode(entity)
	if err != nil {
		return err
	}

	return nil
}

/*ReadBytes - reads a key from the store */
func (ps *Store) ReadBytes(ctx context.Context, key datastore.Key) ([]byte, error) {
	item, ok := ps.DB[key]

	if !ok {
		return nil, datastore.ErrKeyNotFound
	}

	return item, nil
}

/*Write - write an entity to the store */
func (ps *Store) Write(ctx context.Context, entity datastore.Entity) error {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(entity)
	if err != nil {
		return err
	}

	ps.DB[entity.GetKey()] = b.Bytes()

	return nil
}

/*WriteBytes - write bytes to the store */
func (ps *Store) WriteBytes(ctx context.Context, key datastore.Key, value []byte) error {
	ps.DB[key] = value

	return nil
}

func (ps *Store) DeleteKey(ctx context.Context, key datastore.Key) error {

	delete(ps.DB, key)
	return nil
}

/*Delete - Delete an entity from the store */
func (ps *Store) Delete(ctx context.Context, entity datastore.Entity) error {
	return ps.DeleteKey(ctx, entity.GetKey())
}

/*MultiRead - read multiple entities from the store */
func (ps *Store) MultiRead(ctx context.Context, entityMetadata datastore.EntityMetadata, keys []datastore.Key, entities []datastore.Entity) error {
	return nil
}

/*MultiWrite - Write multiple entities to the store */
func (ps *Store) MultiWrite(ctx context.Context, entityMetadata datastore.EntityMetadata, entities []datastore.Entity) error {
	return nil
}

/*MultiDelete - delete multiple entities from the store */
func (ps *Store) MultiDelete(ctx context.Context, entityMetadata datastore.EntityMetadata, entities []datastore.Entity) error {
	// TODO
	return nil
}

func (ps *Store) Iterate(ctx context.Context, handler datastore.StoreIteratorHandler) error {
	for k, v := range ps.DB {
		err := handler(ctx, string(k), v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ps *Store) IteratePrefix(ctx context.Context, prefix string, handler datastore.StoreIteratorHandler) error {
	for k, v := range ps.DB {
		if strings.HasPrefix(k, prefix) {
			err := handler(ctx, string(k), v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (ps *Store) WithReadOnlyConnection(ctx context.Context) context.Context {
	//return context.WithValue(ctx, datastore.CONNECTION_CONTEXT_KEY, ps.GetCon(true))
	return ctx
}

/*WithConnection takes a context and adds a connection value to it */
func (ps *Store) WithConnection(ctx context.Context) context.Context {
	//return context.WithValue(ctx, datastore.CONNECTION_CONTEXT_KEY, ps.GetCon(false))
	return ctx
}

func (ps *Store) Commit(ctx context.Context) error {
	//return ps.GetConnection(ctx).Commit()
	return nil
}

func (ps *Store) Discard(ctx context.Context) {
	//ps.GetConnection(ctx).Discard()
}
