package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/goose"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupDatabase() error {
	fmt.Print("\r> connect data store")
	// check for database connection
	var pgDB *gorm.DB
	var err error
	for i := 0; i < 600; i++ {
		if i > 0 {
			fmt.Printf("\r connect(%v) data store", i)
		}

		err = datastore.GetStore().Open()

		if err == nil {
			pgDB = datastore.GetStore().GetDB()
			break
		}

		if i == 599 { // no more attempts
			logging.Logger.Error("Failed to connect to the database. Shutting the server down")
			return err
		}

		time.Sleep(1 * time.Second)
	}
	var name string
	err = pgDB.Raw("SELECT spcname FROM pg_tablespace where spcname='hdd_tablespace'").Scan(&name).Error
	if err != nil {
		logging.Logger.Error("Failed to check for hdd_archive tablespace", zap.Error(err))
		return err
	}
	if name != "hdd_tablespace" {
		execStr := "CREATE TABLESPACE hdd_tablespace LOCATION '" + config.Configuration.ArchiveDBPath + "'"
		logging.Logger.Info("Creating hdd_archive tablespace", zap.String("path", config.Configuration.ArchiveDBPath), zap.String("execStr", execStr))
		err = pgDB.Exec(execStr).Error
		if err != nil {
			logging.Logger.Error("Failed to create hdd_tablespace", zap.Error(err))
			return err
		}
	}
	if err := migrateDatabase(pgDB); err != nil {
		return fmt.Errorf("error while migrating schema: %v", err)
	}

	return nil
}

func migrateDatabase(db *gorm.DB) error {
	goose.Init()

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	goose.Migrate(sqlDB)
	return nil
}
