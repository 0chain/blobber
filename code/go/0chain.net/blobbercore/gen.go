package blobbercore

//go:generate protoc -I ./blobbergrpc/proto --go-grpc_out=. --go_out=. --grpc-gateway_out=. --openapiv2_out=./openapi ./blobbergrpc/proto/blobber.proto
