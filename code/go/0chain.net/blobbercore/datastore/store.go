package datastore

import (
	"context"
	"fmt"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/errors"
	. "0chain.net/core/logging"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const CONNECTION_CONTEXT_KEY = "connection"

type Store struct {
	db *gorm.DB
}

var store Store

func GetStore() *Store {
	return &store
}

func (store *Store) Open() error {
	db, err := gorm.Open("postgres", fmt.Sprintf("host=%v port=%v user=%v dbname=%v password=%v sslmode=disable", config.Configuration.DBHost, config.Configuration.DBPort, config.Configuration.DBUserName, config.Configuration.DBName, config.Configuration.DBPassword))
	if err != nil {
		return errors.DBOpenError
	}
	store.db = db
	return nil
}

func (store *Store) Close() {
	if store.db != nil {
		store.db.Close()
	}
}

func (store *Store) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, CONNECTION_CONTEXT_KEY, db)
}

func (store *Store) GetTransaction(ctx context.Context) *gorm.DB {
	conn := ctx.Value(CONNECTION_CONTEXT_KEY)
	if conn != nil {
		return conn.(*gorm.DB)
	}
	Logger.Error("No connection in the context.")
	return nil
}

func (store *Store) GetDB() *gorm.DB {
	return store.db
}
