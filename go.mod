module github.com/0chain/blobber

require (
	github.com/0chain/errors v1.0.2
	github.com/0chain/gosdk v1.2.82
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/go-ini/ini v1.55.0 // indirect
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.3.0
	github.com/herumi/bls-go-binary v0.0.0-20191119080710-898950e1a520
	github.com/improbable-eng/grpc-web v0.14.0
	github.com/jackc/pgproto3/v2 v2.0.4 // indirect
	github.com/koding/cache v0.0.0-20161222233015-e8a81b0b3f20
	github.com/minio/minio-go v6.0.14+incompatible
	github.com/mitchellh/mapstructure v1.3.1
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/remeh/sizedwaitgroup v0.0.0-20180822144253-5e7302b12cce
	github.com/rs/cors v1.8.0 // indirect
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	go.uber.org/ratelimit v0.2.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	google.golang.org/genproto v0.0.0-20210303154014-9728d6b83eeb
	google.golang.org/grpc v1.36.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.1
	google.golang.org/protobuf v1.26.0
	gopkg.in/ini.v1 v1.61.0 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gorm.io/datatypes v0.0.0-20200806042100-bc394008dd0d
	gorm.io/driver/postgres v1.0.0
	gorm.io/gorm v1.20.4
	nhooyr.io/websocket v1.8.7 // indirect
)

go 1.13

//replace github.com/0chain/gosdk => ../gosdk
