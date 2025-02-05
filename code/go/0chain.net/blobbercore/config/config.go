package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// SetupDefaultConfig - setup the default config options that can be overridden via the config file
func SetupDefaultConfig() {
	viper.SetDefault("logging.level", "info")

	viper.SetDefault("openconnection_cleaner.tolerance", 3600)
	viper.SetDefault("openconnection_cleaner.frequency", 30)
	viper.SetDefault("writemarker_redeem.frequency", 10)
	viper.SetDefault("writemarker_redeem.num_workers", 5)
	viper.SetDefault("writemarker_redeem.max_chain_length", 32)
	viper.SetDefault("writemarker_redeem.max_timestamp_gap", 1800)
	viper.SetDefault("writemarker_redeem.marker_redeem_interval", time.Minute*10)
	viper.SetDefault("readmarker_redeem.frequency", 10)
	viper.SetDefault("readmarker_redeem.num_workers", 5)
	viper.SetDefault("challenge_response.frequency", 10)
	viper.SetDefault("challenge_response.num_workers", 5)
	viper.SetDefault("challenge_response.max_retries", 10)
	viper.SetDefault("challenge_response.cleanup_gap", 100000)
	viper.SetDefault("rate_limiters.block_limit_daily", 1562500)
	viper.SetDefault("rate_limiters.block_limit_request", 500)
	viper.SetDefault("rate_limiters.block_limit_monthly", 31250000)
	viper.SetDefault("rate_limiters.upload_limit_monthly", 31250000)
	viper.SetDefault("rate_limiters.commit_limit_monthly", 1000000000)
	viper.SetDefault("rate_limiters.commit_limit_daily", 1600)
	viper.SetDefault("rate_limiters.commit_zero_limit_daily", 400)
	viper.SetDefault("rate_limiters.max_connection_changes", 100)

	viper.SetDefault("healthcheck.frequency", "60s")

	viper.SetDefault("capacity", -1)
	viper.SetDefault("read_price", 0.0)
	viper.SetDefault("write_price", 0.0)

	viper.SetDefault("write_marker_lock_timeout", time.Second*30)

	viper.SetDefault("delegate_wallet", "")
	viper.SetDefault("num_delegates", 100)
	viper.SetDefault("service_charge", 0.3)

	viper.SetDefault("update_allocations_interval", time.Duration(-1))
	viper.SetDefault("finalize_allocations_interval", time.Duration(-1))

	viper.SetDefault("max_dirs_files", 50000)
	viper.SetDefault("max_objects_dir", 1000)
}

/*SetupConfig - setup the configuration system */
func SetupConfig(configPath string) {
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()
	viper.SetConfigName("0chain_blobber")

	if configPath == "" {
		viper.AddConfigPath("./config")
	} else {
		viper.AddConfigPath(configPath)
	}

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	Configuration.Config = &config.Configuration
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		ReadConfig(int(Configuration.DeploymentMode))
	})

	viper.WatchConfig()
}

const (
	DeploymentDevelopment = 0
	DeploymentTestNet     = 1
	DeploymentMainNet     = 2
)

type Config struct {
	*config.Config
	DBHost                        string
	PGUserName                    string
	PGPassword                    string
	DBPort                        string
	DBName                        string
	DBUserName                    string
	DBPassword                    string
	DBTablesToKeep                []string
	ArchiveDBPath                 string
	OpenConnectionWorkerFreq      int64
	OpenConnectionWorkerTolerance int64
	WMRedeemFreq                  int64
	WMRedeemNumWorkers            int
	MaxChainLength                int
	MaxTimestampGap               int64
	MarkerRedeemInterval          time.Duration
	RMRedeemFreq                  int64
	RMRedeemNumWorkers            int
	ChallengeResolveFreq          int64
	ChallengeResolveNumWorkers    int
	ChallengeMaxRetires           int
	TempFilesCleanupFreq          int64
	TempFilesCleanupNumWorkers    int
	BlockLimitDaily               int64
	BlockLimitRequest             int64
	BlockLimitMonthly             int64
	UploadLimitMonthly            int64
	CommitLimitMonthly            int64
	CommitLimitDaily              int64
	CommitZeroLimitDaily          int64
	ChallengeCleanupGap           int64
	MaxConnectionChanges          int

	HealthCheckWorkerFreq time.Duration

	ReadPrice  float64
	WritePrice float64
	PriceInUSD bool

	// WriteMarkerLockTimeout lock is released automatically if it is timeout
	WriteMarkerLockTimeout time.Duration

	UpdateAllocationsInterval   time.Duration
	FinalizeAllocationsInterval time.Duration

	MaxAllocationDirFiles int
	MaxObjectsInDir       int

	// DelegateWallet for pool owner.
	DelegateWallet string `json:"delegate_wallet"`
	// NumDelegates maximum allowed.
	NumDelegates int `json:"num_delegates"`
	// ServiceCharge for blobber.
	ServiceCharge float64 `json:"service_charge"`

	// MinSubmit minial submit from miners
	MinSubmit int
	// MinConfirmation minial confirmation from sharders
	MinConfirmation int

	// MountPoint is where allocation files are stored. This is basically arranged in RAID5.
	MountPoint    string
	AllocDirLevel []int
	FileDirLevel  []int
	// AutomacitUpdate Whether to automatically update blobber updates to blockchain
	AutomaticUpdate       bool
	BlobberUpdateInterval time.Duration

	IsEnterprise bool
}

/*Configuration of the system */
var Configuration Config

/*TestNet is the program running in TestNet mode? */
func TestNet() bool {
	return Configuration.DeploymentMode == DeploymentTestNet
}

/*Development - is the programming running in development mode? */
func Development() bool {
	return Configuration.DeploymentMode == DeploymentDevelopment
}

/*ErrSupportedChain error for indicating which chain is supported by the server */
var ErrSupportedChain error

/*MAIN_CHAIN - the main 0chain.net blockchain id */
const MAIN_CHAIN = "0afc093ffb509f059c55478bc1a60351cef7b4e9c008a53a6cc8241ca8617dfe" // TODO:

/*GetMainChainID - get the main chain id */
func GetMainChainID() string {
	return MAIN_CHAIN
}

/*ServerChainID - the chain this server is responsible for */
var ServerChainID = ""

/*SetServerChainID  - set the chain this server is responsible for processing */
func SetServerChainID(chain string) {
	if chain == "" {
		ServerChainID = MAIN_CHAIN
	} else {
		ServerChainID = chain
	}
	ErrSupportedChain = fmt.Errorf("chain %v is not supported by this server", chain)
}

/*GetServerChainID - get the chain this server is responsible for processing */
func GetServerChainID() string {
	if ServerChainID == "" {
		return MAIN_CHAIN
	}
	return ServerChainID
}

/*ValidChain - Is this the chain this server is supposed to process? */
func ValidChain(chain string) error {
	result := chain == ServerChainID || (chain == "" && ServerChainID == MAIN_CHAIN)
	if result {
		return nil
	}
	return ErrSupportedChain
}

func ReadConfig(deploymentMode int) {
	Configuration.AllocDirLevel = viper.GetIntSlice("storage.alloc_dir_level")
	Configuration.FileDirLevel = viper.GetIntSlice("storage.file_dir_level")
	Configuration.DeploymentMode = byte(deploymentMode)
	Configuration.ChainID = viper.GetString("server_chain.id")
	Configuration.SignatureScheme = viper.GetString("server_chain.signature_scheme")

	Configuration.OpenConnectionWorkerFreq = viper.GetInt64("openconnection_cleaner.frequency")
	Configuration.OpenConnectionWorkerTolerance = viper.GetInt64("openconnection_cleaner.tolerance")

	Configuration.WMRedeemFreq = viper.GetInt64("writemarker_redeem.frequency")
	Configuration.WMRedeemNumWorkers = viper.GetInt("writemarker_redeem.num_workers")
	Configuration.MaxChainLength = viper.GetInt("writemarker_redeem.max_chain_length")
	Configuration.MaxTimestampGap = viper.GetInt64("writemarker_redeem.max_timestamp_gap")
	Configuration.MarkerRedeemInterval = viper.GetDuration("writemarker_redeem.marker_redeem_interval")

	Configuration.RMRedeemFreq = viper.GetInt64("readmarker_redeem.frequency")
	Configuration.RMRedeemNumWorkers = viper.GetInt("readmarker_redeem.num_workers")

	Configuration.HealthCheckWorkerFreq = viper.GetDuration("healthcheck.frequency")

	Configuration.ChallengeResolveFreq = viper.GetInt64("challenge_response.frequency")
	Configuration.ChallengeResolveNumWorkers = viper.GetInt("challenge_response.num_workers")
	Configuration.ChallengeMaxRetires = viper.GetInt("challenge_response.max_retries")
	Configuration.ChallengeCleanupGap = viper.GetInt64("challenge_response.cleanup_gap")

	Configuration.AutomaticUpdate = viper.GetBool("disk_update.automatic_update")
	blobberUpdateIntrv := viper.GetDuration("disk_update.blobber_update_interval")
	if blobberUpdateIntrv <= 0 {
		blobberUpdateIntrv = 5 * time.Minute
	}
	Configuration.BlobberUpdateInterval = blobberUpdateIntrv

	Configuration.PGUserName = viper.GetString("pg.user")
	Configuration.PGPassword = viper.GetString("pg.password")
	Configuration.DBHost = viper.GetString("db.host")
	Configuration.DBName = viper.GetString("db.name")
	Configuration.DBPort = viper.GetString("db.port")
	Configuration.DBUserName = viper.GetString("db.user")
	Configuration.DBPassword = viper.GetString("db.password")
	Configuration.DBTablesToKeep = viper.GetStringSlice("db.keep_tables")
	Configuration.ArchiveDBPath = viper.GetString("db.archive_path")
	if Configuration.ArchiveDBPath == "" {
		Configuration.ArchiveDBPath = "/var/lib/postgresql/hdd"
	}

	Configuration.PriceInUSD = viper.GetBool("price_in_usd")

	Configuration.WriteMarkerLockTimeout = viper.GetDuration("write_marker_lock_timeout")

	Configuration.UpdateAllocationsInterval =
		viper.GetDuration("update_allocations_interval")

	Configuration.FinalizeAllocationsInterval =
		viper.GetDuration("finalize_allocations_interval")

	Configuration.MaxAllocationDirFiles =
		viper.GetInt("max_dirs_files")

	Configuration.MaxObjectsInDir = viper.GetInt("max_objects_dir")

	Configuration.DelegateWallet = viper.GetString("delegate_wallet")
	if w := Configuration.DelegateWallet; len(w) != 64 {
		log.Fatal("invalid delegate wallet:", w)
	}

	Configuration.MinSubmit = viper.GetInt("min_submit")
	if Configuration.MinSubmit < 1 {
		Configuration.MinSubmit = 50
	} else if Configuration.MinSubmit > 100 {
		Configuration.MinSubmit = 100
	}
	Configuration.MinConfirmation = viper.GetInt("min_confirmation")
	if Configuration.MinConfirmation < 1 {
		Configuration.MinConfirmation = 50
	} else if Configuration.MinConfirmation > 100 {
		Configuration.MinConfirmation = 100
	}
	Configuration.BlockLimitDaily = viper.GetInt64("rate_limiters.block_limit_daily")
	Configuration.BlockLimitRequest = viper.GetInt64("rate_limiters.block_limit_request")
	Configuration.BlockLimitMonthly = viper.GetInt64("rate_limiters.block_limit_monthly")
	Configuration.UploadLimitMonthly = viper.GetInt64("rate_limiters.upload_limit_monthly")
	Configuration.CommitLimitMonthly = viper.GetInt64("rate_limiters.commit_limit_monthly")
	Configuration.CommitLimitDaily = viper.GetInt64("rate_limiters.commit_limit_daily")
	Configuration.CommitZeroLimitDaily = viper.GetInt64("rate_limiters.commit_zero_limit_daily")

	Configuration.IsEnterprise = viper.GetBool("is_enterprise")
	Configuration.MaxConnectionChanges = viper.GetInt("rate_limiters.max_connection_changes")

	Configuration.IsEnterprise = viper.GetBool("is_enterprise")
}

// StorageSCConfiguration will include all the required sc configs to operate blobber
// If any field it required then it can simply be added in this struct and we are
// good to go
type StorageSCConfiguration struct {
	ChallengeCompletionTime int64
	MaxFileSize             int64
}

var StorageSCConfig StorageSCConfiguration
