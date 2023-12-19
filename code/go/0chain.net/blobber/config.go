package main

import (
	"context"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/spf13/viper"
)

func setupConfig(configDir string, deploymentMode int) {
	fmt.Print("> load config")
	// setup default
	config.SetupDefaultConfig()

	// setup config file
	config.SetupConfig(configDir)

	if mountPoint != "" {
		config.Configuration.MountPoint = mountPoint
	} else {
		config.Configuration.MountPoint = viper.GetString("storage.files_dir")
	}

	if config.Configuration.MountPoint == "" {
		panic("Please specify mount point in flag or config file")
	}
	transaction.MinConfirmation = config.Configuration.MinConfirmation
	config.ReadConfig(deploymentMode)
	fmt.Print("		[OK]\n")
}

func reloadConfig() error {
	fmt.Print("> reload config")

	return datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		s, ok := config.Get(ctx, datastore.GetStore().GetDB())
		if ok {
			if err := s.CopyTo(&config.Configuration); err != nil {
				return err
			}
			fmt.Print("		[OK]\n")
			return nil
		}

		config.Configuration.Capacity = viper.GetInt64("capacity")

		config.Configuration.NumDelegates = viper.GetInt("num_delegates")
		config.Configuration.ReadPrice = viper.GetFloat64("read_price")
		config.Configuration.ServiceCharge = viper.GetFloat64("service_charge")
		config.Configuration.WritePrice = viper.GetFloat64("write_price")

		if err := config.Update(ctx, datastore.GetStore().GetDB()); err != nil {
			return err
		}

		fmt.Print("		[OK]\n")
		return nil

	})
}
