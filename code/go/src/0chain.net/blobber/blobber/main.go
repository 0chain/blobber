package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"0chain.net/blobber"
	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/encryption"
	"0chain.net/logging"
	. "0chain.net/logging"
	"0chain.net/node"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var startTime time.Time
var serverChain *chain.Chain

func initHandlers(r *mux.Router) {

	r.HandleFunc("/", HomePageHandler)
	blobber.SetupHandlers(r)
}

func initEntities() {
	blobber.SetupObjectStorageHandler("./files")
	blobber.SetupProtocol(serverChain)
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

	reader, err := os.Open(*keysFile)
	if err != nil {
		panic(err)
	}

	publicKey, privateKey := encryption.ReadKeys(reader)
	clientId := encryption.Hash(publicKey)
	Logger.Info("The client ID = " + clientId)
	node.Self.SetKeys(publicKey, privateKey)
	reader.Close()
	config.SetServerChainID(config.Configuration.ChainID)

	common.SetupRootContext(node.GetNodeContext())
	//ctx := common.GetRootContext()
	initEntities()
	serverChain = chain.NewChainFromConfig()

	if *nodesFile == "" {
		panic("Please specify --nodes_file file.txt option with a file.txt containing nodes including self")
	}
	logStr := " nodesFile = " + *nodesFile
	Logger.Info(logStr)
	logStr = "Keys file = " + *keysFile + " publicKey = " + publicKey + " privateKey = " + privateKey
	Logger.Info(logStr)

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

	logStr += " Number of miners = " + fmt.Sprintf("%v", len(serverChain.Miners.Nodes))
	logStr += " Number of sharders = " + fmt.Sprintf("%v", len(serverChain.Sharders.Nodes))
	log.Fatal(logStr)

	if node.Self.ID == "" {
		Logger.Panic("node definition for self node doesn't exist")
	} else {
		Logger.Info("self identity", zap.Any("id", node.Self.Node.GetKey()))
	}
	address := fmt.Sprintf(":%v", node.Self.Port)

	chain.SetServerChain(serverChain)

	serverChain.Miners.ComputeProperties()
	serverChain.Sharders.ComputeProperties()
	serverChain.Blobbers.ComputeProperties()
	//miner.GetMinerChain().SetupGenesisBlock(viper.GetString("server_chain.genesis_block.id"))

	mode := "main net"
	if config.Development() {
		mode = "development"
	} else if config.TestNet() {
		mode = "test net"
	}
	Logger.Info("Starting blobber", zap.Int("available_cpus", runtime.NumCPU()), zap.String("port", address), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))

	var server *http.Server
	r := mux.NewRouter()
	if config.Development() {
		// No WriteTimeout setup to enable pprof
		server = &http.Server{
			Addr:           address,
			ReadTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        r, // Pass our instance of gorilla/mux in.
		}
	} else {
		server = &http.Server{
			Addr:           address,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        r, // Pass our instance of gorilla/mux in.
		}
	}
	common.HandleShutdown(server)

	initHandlers(r)
	initServer()

	Logger.Info("Ready to listen to the requests")
	startTime = time.Now().UTC()
	log.Fatal(server.ListenAndServe())
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
