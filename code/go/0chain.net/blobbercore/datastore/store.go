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

type Store interface {
	Open() error
	Close()
	CreateTransaction(ctx context.Context) context.Context
	GetTransaction(ctx context.Context) Transaction
}

var TheStore Store = &store{}

type store struct {
	db *gorm.DB
}

func (store *store) Open() error {
	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Configuration.DBHost, config.Configuration.DBPort,
		config.Configuration.DBUserName, config.Configuration.DBName,
		config.Configuration.DBPassword)), &gorm.Config{})
	if err != nil {
		return errors.DBOpenError
	}

	sqldb, err := db.DB()
	if err != nil {
		return errors.DBOpenError
	}

	sqldb.SetMaxIdleConns(100)
	sqldb.SetMaxOpenConns(200)
	sqldb.SetConnMaxLifetime(30 * time.Second)
	// Enable Logger, show detailed log
	//db.LogMode(true)
	store.db = db
	return nil
}

func (store *store) Close() {
	if store.db != nil {
		if sqldb, _ := store.db.DB(); sqldb != nil {
			sqldb.Close()
		}
	}
}

func (store *store) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, CONNECTION_CONTEXT_KEY, db)
}

func (store *store) GetTransaction(ctx context.Context) Transaction {
	conn := ctx.Value(CONNECTION_CONTEXT_KEY)
	if conn != nil {
		return &transaction{conn.(*gorm.DB)}
	}
	Logger.Error("No connection in the context.")
	return nil
}
