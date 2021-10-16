package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// postgresStore store implementation for postgres
type postgresStore struct {
	db *gorm.DB
}

func (store *postgresStore) Open() error {
	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Configuration.DBHost, config.Configuration.DBPort,
		config.Configuration.DBUserName, config.Configuration.DBName,
		config.Configuration.DBPassword)), &gorm.Config{
		SkipDefaultTransaction: true, // https://gorm.io/docs/performance.html#Disable-Default-Transaction
		PrepareStmt:            true, //https://gorm.io/docs/performance.html#Caches-Prepared-Statement
	})
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

func (store *postgresStore) Close() {
	if store.db != nil {
		if sqldb, _ := store.db.DB(); sqldb != nil {
			sqldb.Close()
		}
	}
}

func (store *postgresStore) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, ContextKeyTransaction, db)
}

func (store *postgresStore) GetTransaction(ctx context.Context) *gorm.DB {
	conn := ctx.Value(ContextKeyTransaction)
	if conn != nil {
		return conn.(*gorm.DB)
	}
	Logger.Error("No connection in the context.")
	return nil
}

func (store *postgresStore) GetDB() *gorm.DB {
	return store.db
}
