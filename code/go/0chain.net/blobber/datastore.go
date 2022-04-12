package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

func setupDatabase(step int) error {
	fmt.Printf("\r[%v/%v] connect data store", step, totalSteps)
	// check for database connection
	for i := 0; i < 600; i++ {
		if i > 0 {
			fmt.Printf("\r[%v/%v] connect(%v) data store", step, totalSteps, i)
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

	fmt.Println("\r	 auto migrate datastore")
	err := datastore.GetStore().AutoMigrate()
	if err != nil {
		logging.Logger.Error("Failed to migrate to the database.")
		return err
	}
	//fmt.Print("	[OK]\n")

	return nil
}
