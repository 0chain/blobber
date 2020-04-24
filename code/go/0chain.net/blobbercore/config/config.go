package config

import (
	"fmt"
	"strings"
	"time"

	"0chain.net/core/config"
	"github.com/spf13/viper"
)

//SetupDefaultConfig - setup the default config options that can be overridden via the config file
func SetupDefaultConfig() {
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("contentref_cleaner.frequency", 30)
	viper.SetDefault("contentref_cleaner.tolerance", 3600)

	viper.SetDefault("openconnection_cleaner.tolerance", 3600)
	viper.SetDefault("openconnection_cleaner.frequency", 30)
	viper.SetDefault("writemarker_redeem.frequency", 10)
	viper.SetDefault("writemarker_redeem.num_workers", 5)
	viper.SetDefault("readmarker_redeem.frequency", 10)
	viper.SetDefault("readmarker_redeem.num_workers", 5)
	viper.SetDefault("challenge_response.frequency", 10)
	viper.SetDefault("challenge_response.num_workers", 5)
	viper.SetDefault("challenge_response.max_retries", 10)

	viper.SetDefault("capacity", -1)
	viper.SetDefault("read_price", 0.0)
	viper.SetDefault("write_price", 0.0)
	viper.SetDefault("min_lock_demand", 0.0)
	viper.SetDefault("max_offer_duration", time.Duration(0))
	viper.SetDefault("challenge_completion_time", time.Duration(-1))
	viper.SetDefault("read_lock_timeout", time.Duration(-1))
	viper.SetDefault("write_lock_timeout", time.Duration(-1))
}

/*SetupConfig - setup the configuration system */
func SetupConfig() {
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()
	viper.SetConfigName("0chain_blobber")
	viper.AddConfigPath("./config")
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
	DBPort                        string
	DBName                        string
	DBUserName                    string
	DBPassword                    string
	ContentRefWorkerFreq          int64
	ContentRefWorkerTolerance     int64
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

	ColdStorageMinimumFileSize  int64
	ColdStorageTimeLimitInHours int64
	ColdStorageJobQueryLimit    int64
	MaxCapacityPercentage       float64

	MinioStart      bool
	MinioWorkerFreq int64
	MinioNumWorkers int
	MinioUseSSL     bool

	ReadPrice               float64
	WritePrice              float64
	MinLockDemand           float64
	MaxOfferDuration        time.Duration
	ChallengeCompletionTime time.Duration

	FaucetWorkerFreqInMinutes int64
	FaucetMinimumBalance      float64

	ReadLockTimeout  int64 // seconds
	WriteLockTimeout int64 // seconds
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
