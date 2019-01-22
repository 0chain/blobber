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

	"0chain.net/allocation"
	"0chain.net/badgerdbstore"
	"0chain.net/blobber"
	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/filestore"
	"0chain.net/logging"
	. "0chain.net/logging"
	"0chain.net/node"
	"0chain.net/reference"
	"0chain.net/transaction"
	"0chain.net/writemarker"

	"0chain.net/readmarker"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var BLOBBER_REGISTERED_LOOKUP_KEY = datastore.ToKey("blobber_registration")

var startTime time.Time
var serverChain *chain.Chain
var filesDir *string
var badgerDir *string

func initHandlers(r *mux.Router) {

	r.HandleFunc("/", HomePageHandler)
	blobber.SetupHandlers(r)
}

func initEntities() {
	badgerdbstore.SetupStorageProvider(*badgerDir)
	fsStore := filestore.SetupFSStore(*filesDir + "/files")

	allocation.SetupAllocationChangeCollectorEntity(badgerdbstore.GetStorageProvider())
	allocation.SetupAllocationEntity(badgerdbstore.GetStorageProvider())
	reference.SetupFileRefEntity(badgerdbstore.GetStorageProvider())
	reference.SetupRefEntity(badgerdbstore.GetStorageProvider())
	blobber.SetupObjectStorageHandler(fsStore, badgerdbstore.GetStorageProvider())
	writemarker.SetupEntity(badgerdbstore.GetStorageProvider())
	readmarker.SetupEntity(badgerdbstore.GetStorageProvider())
	blobber.SetupWorkers(common.GetRootContext())
}

func initServer() {

}

func processBlockChainConfig(nodesFileName string) {
	nodeConfig := viper.New()
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
	config = nodeConfig.Get("blobbers")
	//There shoud be none. Remove it.
	if blobbers, ok := config.([]interface{}); ok {

		serverChain.Blobbers.AddNodes(blobbers)
	}
}

func main() {
	deploymentMode := flag.Int("deployment_mode", 2, "deployment_mode")
	nodesFile := flag.String("nodes_file", "", "nodes_file")
	keysFile := flag.String("keys_file", "", "keys_file")
	maxDelay := flag.Int("max_delay", 0, "max_delay")
	filesDir = flag.String("files_dir", "", "files_dir")
	badgerDir = flag.String("badger_dir", "", "badger_dir")

	flag.Parse()

	config.Configuration.DeploymentMode = byte(*deploymentMode)
	viper.SetDefault("logging.level", "info")

	config.SetupConfig()

	if config.Development() {
		logging.InitLogging("development")
	} else {
		logging.InitLogging("production")
	}
	config.Configuration.ChainID = viper.GetString("server_chain.id")
	config.Configuration.MaxDelay = *maxDelay

	if *filesDir == "" {
		panic("Please specify --files_dir absolute folder name option where uploaded files can be stored")
	}

	if *badgerDir == "" {
		panic("Please specify --badger_dir absolute folder name option where badger db can be stored")
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
			Addr:           address,
			ReadTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        rHandler, // Pass our instance of gorilla/mux in.
		}
	} else {
		server = &http.Server{
			Addr:           address,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        rHandler, // Pass our instance of gorilla/mux in.
		}
	}
	common.HandleShutdown(server)

	initHandlers(r)
	initServer()

	Logger.Info("Ready to listen to the requests")
	startTime = time.Now().UTC()
	log.Fatal(server.ListenAndServe())
}

func RegisterBlobber() {

	registrationRetries := 0
	ctx := badgerdbstore.GetStorageProvider().WithConnection(common.GetRootContext())
	for registrationRetries < 10 {
		txnHash, err := blobber.GetProtocolImpl("").RegisterBlobber(ctx)
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		txnVerified := false
		verifyRetries := 0
		for verifyRetries < transaction.MAX_TXN_RETRIES {
			time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
			t, err := transaction.VerifyTransaction(txnHash, chain.GetServerChain())
			if err == nil {
				txnVerified = true
				Logger.Info("Transaction for adding blobber accepted and verified", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
				badgerdbstore.GetStorageProvider().WriteBytes(ctx, BLOBBER_REGISTERED_LOOKUP_KEY, []byte(txnHash))
				badgerdbstore.GetStorageProvider().Commit(ctx)
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
	fmt.Fprintf(w, "<div>I am a %v with <ul><li>id:%v</li><li>public_key:%v</li></ul></div>\n", node.Self.GetNodeTypeName(), node.Self.GetKey(), node.Self.PublicKey)
	serverChain.Miners.Print(w)
	serverChain.Sharders.Print(w)
	serverChain.Blobbers.Print(w)
}
