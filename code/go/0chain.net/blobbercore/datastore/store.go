package datastore

import (
	"context"
	"fmt"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	. "0chain.net/core/logging"
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
	db, err := gorm.Open("postgres",
		fmt.Sprintf("host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
			config.Configuration.DBHost, config.Configuration.DBPort,
			config.Configuration.DBUserName, config.Configuration.DBName,
			config.Configuration.DBPassword))
	if err != nil {
		return errors.DBOpenError
	}
	db.DB().SetMaxIdleConns(100)
	db.DB().SetMaxOpenConns(200)
	db.DB().SetConnMaxLifetime(30 * time.Second)
	// Enable Logger, show detailed log
	//db.LogMode(true)
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
