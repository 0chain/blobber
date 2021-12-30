module github.com/0chain/blobber

go 1.16

require (
	github.com/0chain/errors v1.0.3
	github.com/0chain/gosdk v1.4.1-0.20211230114941-3d69e6515423
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/didip/tollbooth/v6 v6.1.1
	github.com/go-ini/ini v1.55.0 // indirect
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.6.0
	github.com/herumi/bls-go-binary v1.0.1-0.20210830012634-a8e769d3b872
	github.com/improbable-eng/grpc-web v0.15.0
	github.com/koding/cache v0.0.0-20161222233015-e8a81b0b3f20
	github.com/minio/minio-go v6.0.14+incompatible
	github.com/mitchellh/mapstructure v1.4.2
	github.com/remeh/sizedwaitgroup v0.0.0-20180822144253-5e7302b12cce
	github.com/rs/cors v1.8.0 // indirect
	github.com/selvatico/go-mocket v1.0.7
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	go.uber.org/ratelimit v0.2.0
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20211209193657-4570a0811e8b
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1
	google.golang.org/grpc v1.42.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gorm.io/datatypes v0.0.0-20200806042100-bc394008dd0d
	gorm.io/driver/postgres v1.2.2
	gorm.io/gorm v1.22.3
	nhooyr.io/websocket v1.8.7 // indirect
)

//replace github.com/0chain/gosdk => ../gosdk
