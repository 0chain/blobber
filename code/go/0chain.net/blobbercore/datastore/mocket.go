package datastore

import (
	"context"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	mocket "github.com/selvatico/go-mocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Mocket mock sql driver in data-dog/sqlmock
type Mocket struct {
	db *gorm.DB
}

func (store *Mocket) Open() error {

	mocket.Catcher.Reset()
	mocket.Catcher.Register()
	mocket.Catcher.Logging = true

	dialector := postgres.New(postgres.Config{
		DSN:                  "mockdb",
		DriverName:           mocket.DriverName,
		PreferSimpleProtocol: true,
	})

	gdb, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return err
	}

	store = &Mocket{
		db: gdb,
	}

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
