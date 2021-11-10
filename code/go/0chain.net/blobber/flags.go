package main

import (
	"flag"
	"fmt"
)

var (
	deploymentMode int
	keysFile       string
	minioFile      string
	filesDir       string
	metadataDB     string
	logDir         string
	httpPort       int
	hostname       string
	configDir      string
	grpcPort       int
)

func init() {

	flag.IntVar(&deploymentMode, "deployment_mode", 2, "deployment mode: 0=dev,1=test, 2=mainnet")
	flag.StringVar(&keysFile, "keys_file", "", "keys_file")
	flag.StringVar(&minioFile, "minio_file", "", "minio_file")
	flag.StringVar(&filesDir, "files_dir", "", "files_dir")
	flag.StringVar(&metadataDB, "db_dir", "", "db_dir")
	flag.StringVar(&logDir, "log_dir", "", "log_dir")
	flag.IntVar(&httpPort, "port", 0, "port")
	flag.StringVar(&hostname, "hostname", "", "hostname")
	flag.StringVar(&configDir, "config_dir", "./config", "config_dir")

	flag.IntVar(&grpcPort, "grpc_port", 0, "grpc_port")
}

func parseFlags() {
	fmt.Print("[1/10] load flags")
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

	if httpPort <= 0 {
		panic("Please specify --port which is the port on which requests are accepted")
	}
	fmt.Print("		[OK]\n")
}
