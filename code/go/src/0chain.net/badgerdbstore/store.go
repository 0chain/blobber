package badgerdbstore

import (
	"bytes"
	"context"
	"encoding/json"

	"0chain.net/datastore"
	"github.com/dgraph-io/badger"
)

var storageAPI *Store

/*SetupStorageProvider - sets up badgerDB and opens */
func SetupStorageProvider(badgerDir string) {
	storageAPI = &Store{}
	opts := badger.DefaultOptions
	opts.Dir = badgerDir + "/badgerdb/blobberstate"
	opts.ValueDir = badgerDir + "/badgerdb/blobberstate"
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

/*ReadBytes - reads a key from the store */
func (ps *Store) ReadBytes(ctx context.Context, key datastore.Key) ([]byte, error) {
	resultBytes := make([]byte, 0)
	err := ps.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		resultBytes, err = item.ValueCopy(nil)
		if err != nil {
			return nil
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resultBytes, err
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

/*WriteBytes - write bytes to the store */
func (ps *Store) WriteBytes(ctx context.Context, key datastore.Key, value []byte) error {
	// Start a writable transaction.
	txn := ps.DB.NewTransaction(true)
	defer txn.Discard()

	err := txn.Set([]byte(key), value)
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

func (ps *Store) Iterate(ctx context.Context, handler datastore.StoreIteratorHandler) error {
	err := ps.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			valueBytes, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			handler(ctx, string(k), valueBytes)
		}
		return nil
	})
	return err
}

func (ps *Store) IteratePrefix(ctx context.Context, prefix string, handler datastore.StoreIteratorHandler) error {
	err := ps.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		prefixI := []byte(prefix)
		for it.Seek(prefixI); it.ValidForPrefix(prefixI); it.Next() {
			item := it.Item()
			k := item.Key()
			valueBytes, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			handler(ctx, string(k), valueBytes)
		}
		return nil
	})
	return err
}
