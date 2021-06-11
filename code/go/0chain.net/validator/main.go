package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"0chain.net/core/build"
	"0chain.net/core/chain"

	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"
	"0chain.net/core/util"
	"0chain.net/validatorcore/config"
	"0chain.net/validatorcore/storage"

	"github.com/0chain/gosdk/zcncore"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var startTime time.Time
var serverChain *chain.Chain

func initHandlers(r *mux.Router) {
	r.HandleFunc("/", HomePageHandler)
	storage.SetupHandlers(r)
}

func main() {
	deploymentMode := flag.Int("deployment_mode", 2, "deployment_mode")
	keysFile := flag.String("keys_file", "", "keys_file")
	logDir := flag.String("log_dir", "", "log_dir")
	portString := flag.String("port", "", "port")
	hostname := flag.String("hostname", "", "hostname")
	configDir := flag.String("config_dir", "./config", "config_dir")

	flag.Parse()

	config.SetupDefaultConfig()
	config.SetupConfig(*configDir)

	config.Configuration.DeploymentMode = byte(*deploymentMode)

	if config.Development() {
		logging.InitLogging("development", *logDir, "validator.log")
	} else {
		logging.InitLogging("production", *logDir, "validator.log")
	}

	config.Configuration.ChainID = viper.GetString("server_chain.id")
	config.Configuration.SignatureScheme = viper.GetString("server_chain.signature_scheme")

	// delegate
	config.Configuration.DelegateWallet =
		viper.GetString("delegate_wallet")
	if w := config.Configuration.DelegateWallet; len(w) != 64 {
		log.Fatal("invalid delegate wallet:", w)
	}
	config.Configuration.MinStake = int64(viper.GetFloat64("min_stake") * 1e10)
	config.Configuration.MaxStake = int64(viper.GetFloat64("max_stake") * 1e10)
	config.Configuration.NumDelegates = viper.GetInt("num_delegates")
	config.Configuration.ServiceCharge = viper.GetFloat64("service_charge")

	if *hostname == "" {
		panic("Please specify --hostname which is the public hostname")
	}

	if *portString == "" {
		panic("Please specify --port which is the port on which requests are accepted")
	}

	reader, err := os.Open(*keysFile)
	if err != nil {
		panic(err)
	}

	publicKey, privateKey, _, _ := encryption.ReadKeys(reader)
	reader.Close()
	node.Self.SetKeys(publicKey, privateKey)

	port, err := strconv.Atoi(*portString) //fmt.Sprintf(":%v", port) // node.Self.Port
	if err != nil {
		Logger.Panic("Port specified is not Int " + *portString)
		return
	}

	node.Self.SetHostURL(*hostname, port)
	Logger.Info(" Base URL" + node.Self.GetURLBase())

	config.SetServerChainID(config.Configuration.ChainID)

	common.SetupRootContext(node.GetNodeContext())
	//ctx := common.GetRootContext()
	serverChain = chain.NewChainFromConfig()

	if node.Self.ID == "" {
		Logger.Panic("node definition for self node doesn't exist")
	} else {
		Logger.Info("self identity", zap.Any("id", node.Self.ID))
	}

	//address := publicIP + ":" + portString
	address := ":" + *portString

	chain.SetServerChain(serverChain)

	if err := SetupValidatorOnBC(*logDir); err != nil {
		Logger.Info("error setting up validator on blockchain", zap.Any("err", err))
	}

	mode := "main net"
	if config.Development() {
		mode = "development"
	} else if config.TestNet() {
		mode = "test net"
	}
	Logger.Info("Starting validator", zap.Int("available_cpus", runtime.NumCPU()), zap.String("port", *portString), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))
	var server *http.Server
	r := mux.NewRouter()
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})
	rHandler := handlers.CORS(originsOk, headersOk, methodsOk)(r)
	if config.Development() {
		// No WriteTimeout setup to enable pprof
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			MaxHeaderBytes:    1 << 20,
			Handler:           rHandler, // Pass our instance of gorilla/mux in.
		}
	} else {
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    1 << 20,
			Handler:           rHandler, // Pass our instance of gorilla/mux in.
		}
	}
	common.HandleShutdown(server)

	common.ConfigRateLimits()
	initHandlers(r)

	Logger.Info("Ready to listen to the requests")
	startTime = time.Now().UTC()
	log.Fatal(server.ListenAndServe())
}

func RegisterValidator() {

	registrationRetries := 0
	//ctx := badgerdbstore.GetStorageProvider().WithConnection(common.GetRootContext())
	for registrationRetries < 10 {
		txnHash, err := storage.GetProtocolImpl().RegisterValidator(common.GetRootContext())
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		txnVerified := false
		verifyRetries := 0
		for verifyRetries < util.MAX_RETRIES {
			time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
			t, err := transaction.VerifyTransaction(txnHash, chain.GetServerChain())
			if err == nil {
				Logger.Info("Transaction for adding validator accepted and verified", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
				return
			}
			verifyRetries++
		}

		if !txnVerified {
			Logger.Error("Add validator transaction could not be verified", zap.Any("err", err), zap.String("txn.Hash", txnHash))
		}
	}
}

func SetupValidatorOnBC(logDir string) error {
	var logName = logDir + "/validator.log"
	zcncore.SetLogFile(logName, false)
	zcncore.SetLogLevel(3)
	if err := zcncore.InitZCNSDK(serverChain.BlockWorker, config.Configuration.SignatureScheme); err != nil {
		return err
	}
	if err := zcncore.SetWalletInfo(node.Self.GetWalletString(), false); err != nil {
		return err
	}
	go RegisterValidator()
	return nil
}

/*HomePageHandler - provides basic info when accessing the home page of the server */
func HomePageHandler(w http.ResponseWriter, r *http.Request) {
	mc := chain.GetServerChain()
	fmt.Fprintf(w, "<div>Running since %v ...\n", startTime)
	fmt.Fprintf(w, "<div>Working on the chain: %v</div>\n", mc.ID)
	fmt.Fprintf(w, "<div>I am a validator with <ul><li>id:%v</li><li>public_key:%v</li><li>build_tag:%v</li></ul></div>\n", node.Self.ID, node.Self.PublicKey, build.BuildTag)
	fmt.Fprintf(w, "<div>Miners ...\n")
	network := zcncore.GetNetwork()
	for _, miner := range network.Miners {
		fmt.Fprintf(w, "%v\n", miner)
	}
	fmt.Fprintf(w, "<div>Sharders ...\n")
	for _, sharder := range network.Sharders {
		fmt.Fprintf(w, "%v\n", sharder)
	}
}
