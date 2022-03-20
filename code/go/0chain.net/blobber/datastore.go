package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/automigration"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

func setupDatabase() error {
	fmt.Print("\r[7/12] connect data store")
	// check for database connection
	for i := 0; i < 600; i++ {
		if i > 0 {
			fmt.Printf("\r[7/12] connect(%v) data store", i)
		}

		if err := datastore.GetStore().Open(); err == nil {
			if i == 599 { // no more attempts
				logging.Logger.Error("Failed to connect to the database. Shutting the server down")
				return err
			}
			fmt.Print("	[OK]\n")
			break
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("\r[8/12] auto migrate datastore")
	if err := automigration.MigrateSchema(); err != nil {
		return fmt.Errorf("error while migrating schema: %v", err)
	}

	return nil
}
