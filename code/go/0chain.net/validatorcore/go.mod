module 0chain.net/validatorcore

go 1.14

replace 0chain.net/core => ../core

require (
	0chain.net/core v0.0.0
	github.com/0chain/gosdk v1.0.40
	github.com/gorilla/mux v1.7.3
	github.com/mitchellh/mapstructure v1.1.2
	github.com/pkg/errors v0.8.1 // indirect
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
)
