package datastore

import (
	"context"

	"gorm.io/gorm"
)

type contextKey int

const (
	ContextKeyTransaction contextKey = iota
	ContextKeyStore
)

type CommitToCahe func(tx *EnhancedDB)

type EnhancedDB struct {
	SessionCache     map[string]interface{}
	CommitAllocCache CommitToCahe
	*gorm.DB
}

func EnhanceDB(db *gorm.DB) *EnhancedDB {
	cache := make(map[string]interface{})
	return &EnhancedDB{DB: db, SessionCache: cache}
}

func (edb *EnhancedDB) Commit() *gorm.DB {
	db := edb.DB.Commit()
	if db.Error == nil {
		if edb.CommitAllocCache != nil {
			edb.CommitAllocCache(edb)
		}
	}
	return db
}

type Store interface {

	// GetDB get raw gorm db
	GetDB() *gorm.DB
	// CreateTransaction create transaction, and save it in context
	CreateTransaction(ctx context.Context) context.Context
	// GetTransaction get transaction from context
	GetTransaction(ctx context.Context) *EnhancedDB
	WithNewTransaction(f func(ctx context.Context) error) error
	WithTransaction(ctx context.Context, f func(ctx context.Context) error) error
	// Get db connection with user that creates roles and databases. Its dialactor does not contain database name
	GetPgDB() (*gorm.DB, error)
	Open() error
	Close()
}

var instance Store

func init() {
	instance = &postgresStore{}
}

func GetStore() Store {
	return instance
}

func FromContext(ctx context.Context) Store {
	store := ctx.Value(ContextKeyStore)
	if store != nil {
		return store.(Store)
	}

	return GetStore()
}
