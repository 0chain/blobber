package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/handler"
	"0chain.net/blobbercore/readmarker"
	"0chain.net/blobbercore/writemarker"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"
	"0chain.net/core/util"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//var BLOBBER_REGISTERED_LOOKUP_KEY = datastore.ToKey("blobber_registration")

var startTime time.Time
var serverChain *chain.Chain
var filesDir *string
var metadataDB *string

func initHandlers(r *mux.Router) {
	r.HandleFunc("/", HomePageHandler)
	handler.SetupHandlers(r)
}

func SetupWorkerConfig() {
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

	config.Configuration.Capacity = viper.GetInt64("capacity")

	config.Configuration.DBHost = viper.GetString("db.host")
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
}

func SetupWorkers() {
	// blobber.SetupWorkers(common.GetRootContext())
	// challenge.SetupWorkers(common.GetRootContext(), badgerdbstore.GetStorageProvider(), fsStore)
	readmarker.SetupWorkers(common.GetRootContext())
	writemarker.SetupWorkers(common.GetRootContext())
	// stats.StartEventDispatcher(2)
}

var fsStore filestore.FileStore

func initEntities() {
	// badgerdbstore.SetupStorageProvider(*badgerDir)
	fsStore = filestore.SetupFSStore(*filesDir + "/files")
	// blobber.SetupObjectStorageHandler(fsStore, badgerdbstore.GetStorageProvider())

	// allocation.SetupAllocationChangeCollectorEntity(badgerdbstore.GetStorageProvider())
	// allocation.SetupAllocationEntity(badgerdbstore.GetStorageProvider())
	// allocation.SetupDeleteTokenEntity(badgerdbstore.GetStorageProvider())
	// reference.SetupFileRefEntity(badgerdbstore.GetStorageProvider())
	// reference.SetupRefEntity(badgerdbstore.GetStorageProvider())
	// reference.SetupContentReferenceEntity(badgerdbstore.GetStorageProvider())
	// writemarker.SetupEntity(badgerdbstore.GetStorageProvider())
	// readmarker.SetupEntity(badgerdbstore.GetStorageProvider())
	// challenge.SetupEntity(badgerdbstore.GetStorageProvider())
	// stats.SetupStatsEntity(badgerdbstore.GetStorageProvider())
}

func initServer() {

}

func checkForDBConnection() {
	retries := 0
	var err error
	for retries < 60 {
		err = datastore.GetStore().Open()
		if err != nil {
			time.Sleep(1 * time.Second)
			retries++
			continue
		}
		break
	}

	if err != nil {
		Logger.Error("Error in opening the database. Shutting the server down")
		panic(err)
	}
}

func processBlockChainConfig(nodesFileName string) {
	nodeConfig := viper.New()
	nodeConfig.AddConfigPath("./keysconfig")
	nodeConfig.AddConfigPath("./config")
	nodeConfig.SetConfigName(nodesFileName)

	err := nodeConfig.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
	config := nodeConfig.Get("miners")
	if miners, ok := config.([]interface{}); ok {
		serverChain.Miners.AddNodes(miners)
	}
	config = nodeConfig.Get("sharders")
	if sharders, ok := config.([]interface{}); ok {
		serverChain.Sharders.AddNodes(sharders)
	}
}

func main() {
	deploymentMode := flag.Int("deployment_mode", 2, "deployment_mode")
	nodesFile := flag.String("nodes_file", "", "nodes_file")
	keysFile := flag.String("keys_file", "", "keys_file")
	filesDir = flag.String("files_dir", "", "files_dir")
	metadataDB = flag.String("db_dir", "", "db_dir")
	logDir := flag.String("log_dir", "", "log_dir")

	flag.Parse()

	config.SetupDefaultConfig()
	config.SetupConfig()

	config.Configuration.DeploymentMode = byte(*deploymentMode)

	if config.Development() {
		logging.InitLogging("development", *logDir)
	} else {
		logging.InitLogging("production", *logDir)
	}
	config.Configuration.ChainID = viper.GetString("server_chain.id")
	SetupWorkerConfig()

	if *filesDir == "" {
		panic("Please specify --files_dir absolute folder name option where uploaded files can be stored")
	}

	if *metadataDB == "" {
		panic("Please specify --db_dir absolute folder name option where badger db can be stored")
	}

	reader, err := os.Open(*keysFile)
	if err != nil {
		panic(err)
	}

	publicKey, privateKey, publicIP, portString := encryption.ReadKeys(reader)
	reader.Close()
	node.Self.SetKeys(publicKey, privateKey)

	port, err := strconv.Atoi(portString) //fmt.Sprintf(":%v", port) // node.Self.Port
	if err != nil {
		Logger.Panic("Port specified is not Int " + portString)
		return
	}

	node.Self.SetHostURL(publicIP, port)
	Logger.Info(" Base URL" + node.Self.GetURLBase())

	config.SetServerChainID(config.Configuration.ChainID)

	common.SetupRootContext(node.GetNodeContext())
	//ctx := common.GetRootContext()
	serverChain = chain.NewChainFromConfig()

	if *nodesFile == "" {
		panic("Please specify --nodes_file file.txt option with a file.txt containing nodes including self")
	}

	if strings.HasSuffix(*nodesFile, "txt") {
		reader, err = os.Open(*nodesFile)
		if err != nil {
			log.Fatalf("%v", err)
		}

		node.ReadNodes(reader, serverChain.Miners, serverChain.Sharders, serverChain.Blobbers)
		reader.Close()
	} else { //assumption it has yaml extension
		processBlockChainConfig(*nodesFile)
	}

	if node.Self.ID == "" {
		Logger.Panic("node definition for self node doesn't exist")
	} else {
		Logger.Info("self identity", zap.Any("id", node.Self.Node.GetKey()))
	}

	//address := publicIP + ":" + portString
	address := ":" + portString

	chain.SetServerChain(serverChain)

	serverChain.Miners.ComputeProperties()
	serverChain.Sharders.ComputeProperties()
	serverChain.Blobbers.ComputeProperties()

	checkForDBConnection()

	// Initializa after serverchain is setup.
	initEntities()
	//miner.GetMinerChain().SetupGenesisBlock(viper.GetString("server_chain.genesis_block.id"))
	SetupBlobberOnBC()
	mode := "main net"
	if config.Development() {
		mode = "development"
	} else if config.TestNet() {
		mode = "test net"
	}
	Logger.Info("Starting blobber", zap.Int("available_cpus", runtime.NumCPU()), zap.String("port", portString), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))
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
	handler.HandleShutdown(common.GetRootContext())

	initHandlers(r)
	initServer()

	Logger.Info("Ready to listen to the requests")
	startTime = time.Now().UTC()
	log.Fatal(server.ListenAndServe())
}

func RegisterBlobber() {

	registrationRetries := 0
	//ctx := badgerdbstore.GetStorageProvider().WithConnection(common.GetRootContext())
	for registrationRetries < 10 {
		txnHash, err := handler.RegisterBlobber(common.GetRootContext())
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		txnVerified := false
		verifyRetries := 0
		for verifyRetries < util.MAX_RETRIES {
			time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
			t, err := transaction.VerifyTransaction(txnHash, chain.GetServerChain())
			if err == nil {
				txnVerified = true
				Logger.Info("Transaction for adding blobber accepted and verified", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
				//badgerdbstore.GetStorageProvider().WriteBytes(ctx, BLOBBER_REGISTERED_LOOKUP_KEY, []byte(txnHash))
				//badgerdbstore.GetStorageProvider().Commit(ctx)
				SetupWorkers()
				return
			}
			verifyRetries++
		}

		if !txnVerified {
			Logger.Error("Add blobber transaction could not be verified", zap.Any("err", err), zap.String("txn.Hash", txnHash))
		}
	}
}

func SetupBlobberOnBC() {
	//txnHash, err := badgerdbstore.GetStorageProvider().ReadBytes(common.GetRootContext(), BLOBBER_REGISTERED_LOOKUP_KEY)
	//if err != nil {
	// Now register blobber to chain
	go RegisterBlobber()
	//}
	//Logger.Info("Blobber already registered", zap.Any("blobberTxn", string(txnHash)))
}

/*HomePageHandler - provides basic info when accessing the home page of the server */
func HomePageHandler(w http.ResponseWriter, r *http.Request) {
	mc := chain.GetServerChain()
	fmt.Fprintf(w, "<div>Running since %v ...\n", startTime)
	fmt.Fprintf(w, "<div>Working on the chain: %v</div>\n", mc.ID)
	fmt.Fprintf(w, "<div>I am a blobber with <ul><li>id:%v</li><li>public_key:%v</li></ul></div>\n", node.Self.GetKey(), node.Self.PublicKey)
	serverChain.Miners.Print(w)
	serverChain.Sharders.Print(w)
	serverChain.Blobbers.Print(w)
}
