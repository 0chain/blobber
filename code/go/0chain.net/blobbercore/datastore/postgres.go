package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
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
	logging.Logger.Error("No connection in the context.")
	return nil
}

func (store *postgresStore) GetDB() *gorm.DB {
	return store.db
}

func (store *postgresStore) AutoMigrate() error {

	err := store.db.AutoMigrate(&Migration{})
	if err != nil {
		logging.Logger.Error("[db]", zap.Error(err))
	}

	latest := &Migration{}
	result := store.db.Raw(`select * from "migrations" order by "version" desc limit 1`).First(latest)

	if result.Error != nil {
		if errors.Is(gorm.ErrRecordNotFound, result.Error) {
			latest.Version = "0.0.0"
			latest.CreatedAt = time.Date(2021, 10, 14, 0, 0, 0, 0, time.UTC)
			err = store.db.Create(latest).Error

			if err != nil {
				logging.Logger.Error("[db]"+latest.Version, zap.Error(err))
				return err
			}
		} else {
			logging.Logger.Error("[db]", zap.Error(result.Error))
			return err
		}
	}

	for i := 0; i < len(releases); i++ {
		v := releases[i]
		shouldMigrated, err := v.After(latest)
		if err != nil {
			return err
		}
		if shouldMigrated {
			err = v.Migrate(store.db)
			if err != nil {
				logging.Logger.Error("[db]"+v.Version, zap.Error(err))
				return err
			} else {
				logging.Logger.Info("[db]" + v.Version + " migrated")
			}

		}
	}
	return nil
}
