package badgerdbstore

import (
	"bytes"
	"context"
	"encoding/json"

	"0chain.net/datastore"
	"github.com/dgraph-io/badger"
)

var storageAPI *Store

func SetupStorageProvider() {
	storageAPI = &Store{}
	opts := badger.DefaultOptions
	opts.Dir = "/tmp/badger"
	opts.ValueDir = "/tmp/badger"
	db, err := badger.Open(opts)
	if err != nil {
		panic(err)
	}
	storageAPI.DB = db
}

/*GetStorageProvider - get the storage provider for the memorystore */
func GetStorageProvider() datastore.Store {
	return storageAPI
}

/*Store - just a struct to implement the datastore.Store interface */
type Store struct {
	DB *badger.DB
}

/*Read - read an entity from the store */
func (ps *Store) Read(ctx context.Context, key datastore.Key, entity datastore.Entity) error {

	err := ps.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		valCopy, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		err = json.NewDecoder(bytes.NewReader(valCopy)).Decode(entity)
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

/*Write - write an entity to the store */
func (ps *Store) Write(ctx context.Context, entity datastore.Entity) error {
	// Start a writable transaction.
	txn := ps.DB.NewTransaction(true)
	defer txn.Discard()

	// Use the transaction...
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(entity)
	if err != nil {
		return err
	}

	err = txn.Set([]byte(entity.GetKey()), b.Bytes())
	if err != nil {
		return err
	}

	// Commit the transaction and check for error.
	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

/*Delete - Delete an entity from the store */
func (ps *Store) Delete(ctx context.Context, entity datastore.Entity) error {
	return nil
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
