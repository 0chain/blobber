package datastore

import (
	"context"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var sqlmockInstance *Sqlmock

// UseSqlmock use sqlmock to mock sql driver
func UseSqlmock() {
	if sqlmockInstance == nil {
		sqlmockInstance = &Sqlmock{}
		err := sqlmockInstance.Open()
		if err != nil {
			panic("UseSqlmock: " + err.Error())
		}
	}

	instance = sqlmockInstance
}

// Sqlmock mock sql driver in data-dog/sqlmock
type Sqlmock struct {
	db      *gorm.DB
	Sqlmock sqlmock.Sqlmock
}

func (store *Sqlmock) Open() error {
	db, mock, err := sqlmock.New()
	if err != nil {
		return err
	}

	var dialector = postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})
	var gdb *gorm.DB
	gdb, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return err
	}

	store.db = gdb
	store.Sqlmock = mock

	return nil
}

func (store *Sqlmock) Close() {
	if store.db != nil {
		if db, _ := store.db.DB(); db != nil {
			db.Close()
		}
	}
}

func (store *Sqlmock) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, ContextKeyTransaction, db)
}

func (store *Sqlmock) GetTransaction(ctx context.Context) *gorm.DB {
	conn := ctx.Value(ContextKeyTransaction)
	if conn != nil {
		return conn.(*gorm.DB)
	}
	Logger.Error("No connection in the context.")
	return nil
}

func (store *Sqlmock) GetDB() *gorm.DB {
	return store.db
}

func (store *Sqlmock) AutoMigrate() error {
	return nil
}
