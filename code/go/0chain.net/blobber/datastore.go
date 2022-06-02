package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/automigration"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
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

		pgDB, err = datastore.GetStore().GetPgDB()

		if err == nil {
			break
		}

		if i == 599 { // no more attempts
			logging.Logger.Error("Failed to connect to the database. Shutting the server down")
			return err
		}

		time.Sleep(1 * time.Second)
	}

	if err := automigration.AutoMigrate(pgDB); err != nil {
		return fmt.Errorf("error while migrating schema: %v", err)
	}

	// check for database connection
	for i := 0; i < 600; i++ {
		if i > 0 {
			fmt.Printf("\r connect(%v) data store", i)
		}

		err = datastore.GetStore().Open()

		if err == nil {

			fmt.Print("	[OK]\n")
			break
		}

		if i == 599 { // no more attempts
			logging.Logger.Error("Failed to connect to the database. Shutting the server down")
			return err
		}

		time.Sleep(1 * time.Second)
	}

	return automigration.AddVersion(config.Configuration.Version)
}
