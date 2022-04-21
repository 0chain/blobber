package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	deploymentMode    int
	keysFile          string
	minioFile         string
	filesDir          string
	metadataDB        string
	logDir            string
	httpPort          int
	hostname          string
	configDir         string
	grpcPort          int
	isIntegrationTest bool
	httpsPort         int
	httpsKeyFile      string
	httpsCertFile     string
	hostUrl           string
)

func init() {
	flag.IntVar(&deploymentMode, "deployment_mode", 2, "deployment mode: 0=dev,1=test, 2=mainnet")
	flag.StringVar(&keysFile, "keys_file", "", "keys_file")
	flag.StringVar(&minioFile, "minio_file", "", "minio_file")
	flag.StringVar(&filesDir, "files_dir", "", "files_dir")
	flag.StringVar(&metadataDB, "db_dir", "", "db_dir")
	flag.StringVar(&logDir, "log_dir", "", "log_dir")
	flag.IntVar(&httpPort, "port", 0, "port")
	flag.IntVar(&httpsPort, "https_port", 0, "https_port")
	flag.StringVar(&httpsCertFile, "https_cert_file", "", "https_cert_file")
	flag.StringVar(&httpsKeyFile, "https_key_file", "", "https_key_file")
	flag.StringVar(&hostname, "hostname", "", "hostname")
	flag.StringVar(&configDir, "config_dir", "./config", "config_dir")

	flag.StringVar(&hostUrl, "hosturl", "", "register url on blockchain instead of [schema://hostname+port] if it has value")

	flag.IntVar(&grpcPort, "grpc_port", 0, "grpc_port")
}

func parseFlags() {
	fmt.Print("> load flags")
	flag.Parse()

	if filesDir == "" {
		panic("Please specify --files_dir absolute folder name option where uploaded files can be stored")
	}

	if metadataDB == "" {
		panic("Please specify --db_dir absolute folder name option where meta data db can be stored")
	}

	if hostname == "" {
		panic("Please specify --hostname which is the public hostname")
	}

	if httpPort <= 0 && httpsPort <= 0 {
		panic("Please specify --port or --https-port which is the port on which requests are accepted")
	}
	isIntegrationTest = os.Getenv("integration") == "1"

	if httpsPort > 0 && (httpsCertFile == "" || httpsKeyFile == "") {
		panic("Please specify --https-cert-file and --https-key-file if you are using --https-port")
	}

	fmt.Print("		[OK]\n")
}
