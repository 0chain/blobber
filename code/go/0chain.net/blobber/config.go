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

	config.Configuration.ContentRefWorkerFreq = viper.GetInt64("contentref_cleaner.frequency")
	config.Configuration.ContentRefWorkerTolerance = viper.GetInt64("contentref_cleaner.tolerance")

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

	config.Configuration.ColdStorageMinimumFileSize = viper.GetInt64("cold_storage.min_file_size")
	config.Configuration.ColdStorageTimeLimitInHours = viper.GetInt64("cold_storage.file_time_limit_in_hours")
	config.Configuration.ColdStorageJobQueryLimit = viper.GetInt("cold_storage.job_query_limit")
	config.Configuration.ColdStorageStartCapacitySize = viper.GetUint64("cold_storage.start_capacity_size")
	config.Configuration.ColdStorageDeleteLocalCopy = viper.GetBool("cold_storage.delete_local_copy")
	config.Configuration.ColdStorageDeleteCloudCopy = viper.GetBool("cold_storage.delete_cloud_copy")

	config.Configuration.MinioStart = viper.GetBool("minio.start")
	config.Configuration.MinioWorkerFreq = viper.GetInt64("minio.worker_frequency")
	config.Configuration.MinioUseSSL = viper.GetBool("minio.use_ssl")
	config.Configuration.MinioStorageUrl = viper.GetString("minio.storage_service_url")
	config.Configuration.MinioAccessID = viper.GetString("minio.access_id")
	config.Configuration.MinioSecretKey = viper.GetString("minio.secret_access_key")
	config.Configuration.MinioBucket = viper.GetString("minio.bucket_name")
	config.Configuration.MinioRegion = viper.GetString("minio.region")

	config.Configuration.PGUserName = viper.GetString("pg.user")
	config.Configuration.PGPassword = viper.GetString("pg.password")
	config.Configuration.DBHost = viper.GetString("db.host")
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
	config.Configuration.DBTablesToKeep = viper.GetStringSlice("db.keep_tables")

	config.Configuration.PriceInUSD = viper.GetBool("price_in_usd")

	config.Configuration.ReadLockTimeout = int64(
		viper.GetDuration("read_lock_timeout") / time.Second,
	)
	config.Configuration.WriteLockTimeout = int64(
		viper.GetDuration("write_lock_timeout") / time.Second,
	)

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

	config.Configuration.Name = viper.GetString("info.name")
	config.Configuration.WebsiteUrl = viper.GetString("info.website_url")
	config.Configuration.LogoUrl = viper.GetString("info.logo_url")
	config.Configuration.Description = viper.GetString("info.description")

	config.Configuration.Geolocation = config.GeolocationConfig{}
	config.Configuration.Geolocation.Latitude = viper.GetFloat64("geolocation.latitude")
	config.Configuration.Geolocation.Longitude = viper.GetFloat64("geolocation.longitude")

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

	config.Configuration.MaxOfferDuration = viper.GetDuration("max_offer_duration")
	config.Configuration.MaxStake = int64(viper.GetFloat64("max_stake") * 1e10)
	config.Configuration.MinLockDemand = viper.GetFloat64("min_lock_demand")
	config.Configuration.MinStake = int64(viper.GetFloat64("min_stake") * 1e10)
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
