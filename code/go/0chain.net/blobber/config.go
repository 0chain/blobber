package main

import (
	"context"
	"fmt"
	"log"
	"time"

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
	config.Configuration.AllocDirLevel = viper.GetIntSlice("storage.alloc_dir_level")
	config.Configuration.FileDirLevel = viper.GetIntSlice("storage.file_dir_level")
	config.Configuration.DeploymentMode = byte(deploymentMode)
	config.Configuration.ChainID = viper.GetString("server_chain.id")
	config.Configuration.SignatureScheme = viper.GetString("server_chain.signature_scheme")

	config.Configuration.OpenConnectionWorkerFreq = viper.GetInt64("openconnection_cleaner.frequency")
	config.Configuration.OpenConnectionWorkerTolerance = viper.GetInt64("openconnection_cleaner.tolerance")

	config.Configuration.WMRedeemFreq = viper.GetInt64("writemarker_redeem.frequency")
	config.Configuration.WMRedeemNumWorkers = viper.GetInt("writemarker_redeem.num_workers")

	config.Configuration.RMRedeemFreq = viper.GetInt64("readmarker_redeem.frequency")
	config.Configuration.RMRedeemNumWorkers = viper.GetInt("readmarker_redeem.num_workers")

	config.Configuration.HealthCheckWorkerFreq = viper.GetDuration("healthcheck.frequency")

	config.Configuration.ChallengeResolveFreq = viper.GetInt64("challenge_response.frequency")
	config.Configuration.ChallengeResolveNumWorkers = viper.GetInt("challenge_response.num_workers")
	config.Configuration.ChallengeMaxRetires = viper.GetInt("challenge_response.max_retries")

	config.Configuration.AutomaticUpdate = viper.GetBool("disk_update.automatic_update")
	blobberUpdateIntrv := viper.GetDuration("disk_update.blobber_update_interval")
	if blobberUpdateIntrv <= 0 {
		blobberUpdateIntrv = 5 * time.Minute
	}
	config.Configuration.BlobberUpdateInterval = blobberUpdateIntrv

	config.Configuration.PGUserName = viper.GetString("pg.user")
	config.Configuration.PGPassword = viper.GetString("pg.password")
	config.Configuration.DBHost = viper.GetString("db.host")
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
	config.Configuration.DBTablesToKeep = viper.GetStringSlice("db.keep_tables")

	config.Configuration.PriceInUSD = viper.GetBool("price_in_usd")

	config.Configuration.WriteMarkerLockTimeout = viper.GetDuration("write_marker_lock_timeout")

	config.Configuration.UpdateAllocationsInterval =
		viper.GetDuration("update_allocations_interval")

	config.Configuration.MaxAllocationDirFiles =
		viper.GetInt("max_dirs_files")
	if config.Configuration.MaxAllocationDirFiles < 50000 {
		config.Configuration.MaxAllocationDirFiles = 50000
	}

	config.Configuration.DelegateWallet = viper.GetString("delegate_wallet")
	if w := config.Configuration.DelegateWallet; len(w) != 64 {
		log.Fatal("invalid delegate wallet:", w)
	}

	config.Configuration.MinSubmit = viper.GetInt("min_submit")
	if config.Configuration.MinSubmit < 1 {
		config.Configuration.MinSubmit = 50
	} else if config.Configuration.MinSubmit > 100 {
		config.Configuration.MinSubmit = 100
	}
	config.Configuration.MinConfirmation = viper.GetInt("min_confirmation")
	if config.Configuration.MinConfirmation < 1 {
		config.Configuration.MinConfirmation = 50
	} else if config.Configuration.MinConfirmation > 100 {
		config.Configuration.MinConfirmation = 100
	}

	transaction.MinConfirmation = config.Configuration.MinConfirmation

	fmt.Print("		[OK]\n")
}

func reloadConfig() error {
	fmt.Print("> reload config")

	db := datastore.GetStore().GetDB()

	s, ok := config.Get(context.TODO(), db)
	if ok {
		if err := s.CopyTo(&config.Configuration); err != nil {
			return err
		}
		fmt.Print("		[OK]\n")
		return nil
	}

	config.Configuration.Capacity = viper.GetInt64("capacity")

	config.Configuration.MinLockDemand = viper.GetFloat64("min_lock_demand")
	config.Configuration.NumDelegates = viper.GetInt("num_delegates")
	config.Configuration.ReadPrice = viper.GetFloat64("read_price")
	config.Configuration.ServiceCharge = viper.GetFloat64("service_charge")
	config.Configuration.WritePrice = viper.GetFloat64("write_price")

	if err := config.Update(context.TODO(), db); err != nil {
		return err
	}

	fmt.Print("		[OK]\n")
	return nil
}
