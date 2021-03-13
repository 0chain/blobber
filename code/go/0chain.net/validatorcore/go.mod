module 0chain.net/validatorcore

go 1.14

replace 0chain.net/core => ../core

require (
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.85
	github.com/gorilla/mux v1.7.3
	github.com/mitchellh/mapstructure v1.1.2
	github.com/pkg/errors v0.8.1 // indirect
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
)
