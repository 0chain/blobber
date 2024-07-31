package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"moul.io/zapgorm2"
)

// postgresStore store implementation for postgres
type postgresStore struct {
	db *gorm.DB
}

func (p *postgresStore) GetPgDB() (*gorm.DB, error) {

	db, err := gorm.Open(postgres.Open(
		fmt.Sprintf("host=%v port=%v user=%v password=%v sslmode=disable",
			config.Configuration.DBHost,
			config.Configuration.DBPort,
			config.Configuration.PGUserName,
			config.Configuration.PGPassword),
	))

	if err != nil {
		return nil, err
	}

	sqldb, err := db.DB()
	if err != nil {
		return nil, common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	if err := sqldb.Ping(); err != nil {
		return nil, common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	return db, err

}

func (store *postgresStore) Open() error {
	gormLogger := zapgorm2.New(logging.Logger)
	gormLogger.SlowThreshold = 100 * time.Millisecond
	gormLogger.IgnoreRecordNotFoundError = true
	gormLogger.SkipCallerLookup = true
	gormLogger.SetAsDefault()
	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Configuration.DBHost, config.Configuration.DBPort,
		config.Configuration.DBUserName, config.Configuration.DBName,
		config.Configuration.DBPassword)), &gorm.Config{
		SkipDefaultTransaction: true, // https://gorm.io/docs/performance.html#Disable-Default-Transaction
		PrepareStmt:            true, //https://gorm.io/docs/performance.html#Caches-Prepared-Statement
		Logger:                 gormLogger,
	})
	if err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqldb, err := db.DB()
	if err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	if err := sqldb.Ping(); err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqldb.SetMaxIdleConns(100)
	sqldb.SetMaxOpenConns(400)
	sqldb.SetConnMaxLifetime(30 * time.Minute)
	sqldb.SetConnMaxIdleTime(5 * time.Minute)
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

func (store *postgresStore) CreateTransaction(ctx context.Context, opts ...*sql.TxOptions) context.Context {
	db := store.db.WithContext(ctx).Begin()
	return context.WithValue(ctx, ContextKeyTransaction, EnhanceDB(db))
}

func (store *postgresStore) GetTransaction(ctx context.Context) *EnhancedDB {
	conn := ctx.Value(ContextKeyTransaction)
	if conn != nil {
		return conn.(*EnhancedDB)
	}
	logging.Logger.Error("No connection in the context.")
	return nil
}

func (store *postgresStore) WithNewTransaction(f func(ctx context.Context) error, opts ...*sql.TxOptions) error {
	timeoutctx, cancel := context.WithTimeout(context.TODO(), 45*time.Second)
	defer cancel()
	ctx := store.CreateTransaction(timeoutctx)
	tx := store.GetTransaction(ctx)
	if tx.Error != nil {
		return tx.Error
	}
	err := f(ctx)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		return err
	}
	return nil
}
func (store *postgresStore) WithTransaction(ctx context.Context, f func(ctx context.Context) error) error {
	tx := store.GetTransaction(ctx)
	if tx == nil {
		ctx = store.CreateTransaction(ctx)
		tx = store.GetTransaction(ctx)
		if tx.Error != nil {
			return tx.Error
		}
	}

	err := f(ctx)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		return err
	}
	return nil
}

func (store *postgresStore) GetDB() *gorm.DB {
	return store.db
}
