package datastore

import (
	"context"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	mocket "github.com/selvatico/go-mocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var mocketInstance *Mocket

// UseMocket use mocket to mock sql driver
func UseMocket(logging bool) {
	if mocketInstance == nil {
		mocketInstance = &Mocket{}
		mocketInstance.logging = logging
		mocketInstance.Open()
	}

	instance = mocketInstance
}

// Mocket mock sql driver in data-dog/sqlmock
type Mocket struct {
	logging bool
	db      *gorm.DB
}

func (store *Mocket) Open() error {

	mocket.Catcher.Reset()
	mocket.Catcher.Register()
	mocket.Catcher.Logging = store.logging

	dialector := postgres.New(postgres.Config{
		DSN:                  "mockdb",
		DriverName:           mocket.DriverName,
		PreferSimpleProtocol: true,
	})

	cfg := &gorm.Config{}

	if !store.logging {
		cfg.Logger = logger.Default.LogMode(logger.Silent)
	}

	gdb, err := gorm.Open(dialector, cfg)
	if err != nil {
		return err
	}

	store.db = gdb

	return nil
}

func (store *Mocket) Close() {
	if store.db != nil {

		if db, _ := store.db.DB(); db != nil {
			db.Close()
		}
	}
}

func (store *Mocket) CreateTransaction(ctx context.Context) context.Context {
	db := store.db.Begin()
	return context.WithValue(ctx, ContextKeyTransaction, db)
}

func (store *Mocket) GetTransaction(ctx context.Context) *gorm.DB {
	conn := ctx.Value(ContextKeyTransaction)
	if conn != nil {
		return conn.(*gorm.DB)
	}
	Logger.Error("No connection in the context.")
	return nil
}

func (store *Mocket) GetDB() *gorm.DB {
	return store.db
}
