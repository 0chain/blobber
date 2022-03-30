package main

import (
	"fmt"
	"log"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/spf13/viper"
)

func setupConfig() {
	fmt.Print("[2/12] load config")
	// setup default
	config.SetupDefaultConfig()

	// setup config file
	config.SetupConfig(configDir)

	// load config
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

	config.Configuration.ChallengeResolveFreq = viper.GetInt64("challenge_response.frequency")
	config.Configuration.ChallengeResolveNumWorkers = viper.GetInt("challenge_response.num_workers")
	config.Configuration.ChallengeMaxRetires = viper.GetInt("challenge_response.max_retries")

	config.Configuration.ColdStorageMinimumFileSize = viper.GetInt64("cold_storage.min_file_size")
	config.Configuration.ColdStorageTimeLimitInHours = viper.GetInt64("cold_storage.file_time_limit_in_hours")
	config.Configuration.ColdStorageJobQueryLimit = viper.GetInt64("cold_storage.job_query_limit")
	config.Configuration.ColdStorageStartCapacitySize = viper.GetInt64("cold_storage.start_capacity_size")
	config.Configuration.ColdStorageDeleteLocalCopy = viper.GetBool("cold_storage.delete_local_copy")
	config.Configuration.ColdStorageDeleteCloudCopy = viper.GetBool("cold_storage.delete_cloud_copy")

	config.Configuration.MinioStart = viper.GetBool("minio.start")
	config.Configuration.MinioWorkerFreq = viper.GetInt64("minio.worker_frequency")
	config.Configuration.MinioUseSSL = viper.GetBool("minio.use_ssl")

	config.Configuration.Capacity = viper.GetInt64("capacity")

	config.Configuration.PGUserName = viper.GetString("pg.user")
	config.Configuration.PGPassword = viper.GetString("pg.password")
	config.Configuration.DBHost = viper.GetString("db.host")
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
	config.Configuration.DBDropAllTables = viper.GetBool("db.drop_all")
	config.Configuration.DBTablesToDrop = viper.GetStringSlice("db.drop_tables")
	config.Configuration.DBTablesToKeep = viper.GetStringSlice("db.except_tables")

	config.Configuration.Capacity = viper.GetInt64("capacity")
	config.Configuration.ReadPrice = viper.GetFloat64("read_price")
	config.Configuration.WritePrice = viper.GetFloat64("write_price")
	config.Configuration.PriceInUSD = viper.GetBool("price_in_usd")
	config.Configuration.MinLockDemand = viper.GetFloat64("min_lock_demand")
	config.Configuration.MaxOfferDuration = viper.GetDuration("max_offer_duration")
	config.Configuration.ChallengeCompletionTime = viper.GetDuration("challenge_completion_time")

	config.Configuration.ReadLockTimeout = int64(
		viper.GetDuration("read_lock_timeout") / time.Second,
	)
	config.Configuration.WriteLockTimeout = int64(
		viper.GetDuration("write_lock_timeout") / time.Second,
	)

	config.Configuration.WriteMarkerLockTimeout = viper.GetDuration("write_marker_lock_timeout")

	config.Configuration.UpdateAllocationsInterval =
		viper.GetDuration("update_allocations_interval")

	config.Configuration.DelegateWallet = viper.GetString("delegate_wallet")
	if w := config.Configuration.DelegateWallet; len(w) != 64 {
		log.Fatal("invalid delegate wallet:", w)
	}
	config.Configuration.MinStake = int64(viper.GetFloat64("min_stake") * 1e10)
	config.Configuration.MaxStake = int64(viper.GetFloat64("max_stake") * 1e10)
	config.Configuration.NumDelegates = viper.GetInt("num_delegates")
	config.Configuration.ServiceCharge = viper.GetFloat64("service_charge")

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

	config.Configuration.Name = viper.GetString("name")
	config.Configuration.WebsiteUrl = viper.GetString("website_url")
	config.Configuration.LogoUrl = viper.GetString("logo_url")
	config.Configuration.Description = viper.GetString("description")

	fmt.Print("		[OK]\n")
}
