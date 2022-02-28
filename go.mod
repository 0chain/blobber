module github.com/0chain/blobber

go 1.16

// temporary, for development
// replace github.com/0chain/gosdk => ../gosdk

require (
	github.com/0chain/errors v1.0.3
	github.com/0chain/gosdk v1.7.3-0.20220228071521-f3ed1e982a40
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/didip/tollbooth/v6 v6.1.2
	github.com/go-ini/ini v1.55.0 // indirect
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.3
	github.com/herumi/bls-go-binary v1.0.1-0.20210830012634-a8e769d3b872
	github.com/improbable-eng/grpc-web v0.15.0
	github.com/jackc/pgx/v4 v4.15.0 // indirect
	github.com/koding/cache v0.0.0-20161222233015-e8a81b0b3f20
	github.com/minio/minio-go v6.0.14+incompatible
	github.com/mitchellh/mapstructure v1.4.3
	github.com/remeh/sizedwaitgroup v0.0.0-20180822144253-5e7302b12cce
	github.com/rs/cors v1.8.0 // indirect
	github.com/selvatico/go-mocket v1.0.7
	github.com/spf13/afero v1.7.1 // indirect
	github.com/spf13/viper v1.10.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/ratelimit v0.2.0
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	google.golang.org/genproto v0.0.0-20220208230804-65c12eb4c068
	google.golang.org/grpc v1.44.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gorm.io/datatypes v0.0.0-20200806042100-bc394008dd0d
	gorm.io/driver/postgres v1.3.1
	gorm.io/gorm v1.23.1
	nhooyr.io/websocket v1.8.7 // indirect
)

//replace github.com/0chain/gosdk => ../gosdk
