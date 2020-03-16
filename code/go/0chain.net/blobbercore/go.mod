module 0chain.net/blobbercore

replace 0chain.net/core => ../core

require (
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.36
	github.com/gorilla/mux v1.6.2
	github.com/jinzhu/gorm v1.9.8
	github.com/lib/pq v1.1.1 // indirect
	github.com/remeh/sizedwaitgroup v0.0.0-20180822144253-5e7302b12cce
	github.com/spf13/viper v1.4.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
)

go 1.13
