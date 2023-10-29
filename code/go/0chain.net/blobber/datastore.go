package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/goose"
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
