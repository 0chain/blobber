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

func setupDatabase(step int) error {
	fmt.Printf("\r[%v/%v] connect data store", step, totalSteps)
	// check for database connection
	var pgDB *gorm.DB
	var err error
	for i := 0; i < 600; i++ {
		if i > 0 {
			fmt.Printf("\r[%v/%v] connect(%v) data store", step, totalSteps, i)
		}

		pgDB, err = datastore.GetStore().GetPgDB()
		if err == nil && pgDB != nil {
			break
		}

		if i == 599 {
			logging.Logger.Error("Failed to connect to the database. Shutting the server down")
			return fmt.Errorf("could not get postgres db connection. Error: %v", err)
		}

		time.Sleep(1 * time.Second)
		fmt.Print("	[OK]\n")

	}

	if !config.Configuration.DBAutoMigrate {
		logging.Logger.Info("Automigration is skipped")
		return nil
	}

	if err := automigration.AutoMigrate(pgDB); err != nil {
		return fmt.Errorf("error while migrating schema: %v", err)
	}

	return nil
}
