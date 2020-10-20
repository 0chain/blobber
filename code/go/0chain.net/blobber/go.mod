module blobber

replace 0chain.net/core => ../core

replace 0chain.net/blobbercore => ../blobbercore

replace 0chain.net/conductor => ../conductor

require (
	0chain.net/blobbercore v0.0.0
	0chain.net/conductor v0.0.0-00010101000000-000000000000
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.85
	github.com/didip/tollbooth v4.0.2+incompatible // indirect
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.3
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/spf13/viper v1.7.0
	go.uber.org/zap v1.15.0
)

go 1.13
