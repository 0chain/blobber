module blobber

replace 0chain.net/core => ../core

replace 0chain.net/blobbercore => ../blobbercore

require (
	0chain.net/blobbercore v0.0.0
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.31
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.3
	github.com/spf13/viper v1.4.0
	go.uber.org/zap v1.10.0
)
