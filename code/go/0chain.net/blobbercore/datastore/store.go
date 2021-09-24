package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type contextKey int

const CONNECTION_CONTEXT_KEY contextKey = iota

type Store struct {
	db *gorm.DB
}

var store Store

func setDB(db *gorm.DB) {
	store.db = db
}

func GetStore() *Store {
	return &store
}

func (store *Store) Open() error {
	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Configuration.DBHost, config.Configuration.DBPort,
		config.Configuration.DBUserName, config.Configuration.DBName,
		config.Configuration.DBPassword)), &gorm.Config{})
	if err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqldb, err := db.DB()
	if err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqldb.SetMaxIdleConns(100)
	sqldb.SetMaxOpenConns(200)
	sqldb.SetConnMaxLifetime(30 * time.Second)
	// Enable Logger, show detailed log
	//db.LogMode(true)
	store.db = db
	return nil
}

func (store *Store) Close() {
	if store.db != nil {
		if sqldb, _ := store.db.DB(); sqldb != nil {
			sqldb.Close()
		}
	}
}

func (store *Store) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, CONNECTION_CONTEXT_KEY, db) //nolint:staticcheck // changing type might require further refactor
}

func (store *Store) GetTransaction(ctx context.Context) *gorm.DB {
	conn := ctx.Value(CONNECTION_CONTEXT_KEY)
	if conn != nil {
		return conn.(*gorm.DB)
	}
	Logger.Error("No connection in the context.")
	return nil
}

func (store *Store) HasTransaction(ctx context.Context) bool {
	conn := ctx.Value(CONNECTION_CONTEXT_KEY)
	return conn != nil
}

func (store *Store) GetDB() *gorm.DB {
	return store.db
}
