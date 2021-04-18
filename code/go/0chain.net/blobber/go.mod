module blobber

replace 0chain.net/core => ../core

replace 0chain.net/blobbercore => ../blobbercore

replace 0chain.net/conductor => ../conductor

require (
	0chain.net/blobbercore v0.0.0
	0chain.net/conductor v0.0.0-00010101000000-000000000000
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.1.6
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.3
	github.com/spf13/viper v1.7.0
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	golang.org/x/tools v0.0.0-20200207183749-b753a1ba74fa // indirect
	google.golang.org/grpc v1.33.1
)

go 1.13
