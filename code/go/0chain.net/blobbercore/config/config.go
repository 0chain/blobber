package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/spf13/viper"
)

// SetupDefaultConfig - setup the default config options that can be overridden via the config file
func SetupDefaultConfig() {
	viper.SetDefault("logging.level", "info")

	viper.SetDefault("openconnection_cleaner.tolerance", 3600)
	viper.SetDefault("openconnection_cleaner.frequency", 30)
	viper.SetDefault("writemarker_redeem.frequency", 10)
	viper.SetDefault("writemarker_redeem.num_workers", 5)
	viper.SetDefault("readmarker_redeem.frequency", 10)
	viper.SetDefault("readmarker_redeem.num_workers", 5)
	viper.SetDefault("challenge_response.frequency", 10)
	viper.SetDefault("challenge_response.num_workers", 5)
	viper.SetDefault("challenge_response.max_retries", 10)
	viper.SetDefault("rate_limiters.block_limit_daily", 1562500)
	viper.SetDefault("rate_limiters.block_limit_request", 500)
	viper.SetDefault("rate_limiters.block_limit_monthly", 31250000)
	viper.SetDefault("rate_limiters.upload_limit_monthly", 31250000)
	viper.SetDefault("rate_limiters.commit_limit_monthly", 30000)

	viper.SetDefault("healthcheck.frequency", "60s")

	viper.SetDefault("capacity", -1)
	viper.SetDefault("read_price", 0.0)
	viper.SetDefault("write_price", 0.0)
	viper.SetDefault("min_lock_demand", 0.0)

	viper.SetDefault("write_marker_lock_timeout", time.Second*30)

	viper.SetDefault("delegate_wallet", "")
	viper.SetDefault("num_delegates", 100)
	viper.SetDefault("service_charge", 0.3)

	viper.SetDefault("update_allocations_interval", time.Duration(-1))
	viper.SetDefault("finalize_allocations_interval", time.Duration(-1))
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
	OpenConnectionWorkerFreq      int64
	OpenConnectionWorkerTolerance int64
	WMRedeemFreq                  int64
	WMRedeemNumWorkers            int
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

	HealthCheckWorkerFreq time.Duration

	ReadPrice  float64
	WritePrice float64
	PriceInUSD bool

	// WriteMarkerLockTimeout lock is released automatically if it is timeout
	WriteMarkerLockTimeout time.Duration

	UpdateAllocationsInterval   time.Duration
	FinalizeAllocationsInterval time.Duration

	MaxAllocationDirFiles int

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

// StorageSCConfiguration will include all the required sc configs to operate blobber
// If any field it required then it can simply be added in this struct and we are
// good to go
type StorageSCConfiguration struct {
	ChallengeCompletionTime int64
	MaxFileSize             int64
}

var StorageSCConfig StorageSCConfiguration
