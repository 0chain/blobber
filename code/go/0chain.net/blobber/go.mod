module blobber

replace 0chain.net/core => ../core

replace 0chain.net/blobbercore => ../blobbercore

require (
	0chain.net/blobbercore v0.0.0
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.85
	github.com/go-ini/ini v1.55.0 // indirect
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.3
	github.com/minio/minio-go v6.0.14+incompatible // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/spf13/viper v1.4.0
	go.uber.org/zap v1.10.0
)

go 1.13
