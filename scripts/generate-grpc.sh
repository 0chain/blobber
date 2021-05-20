#!/usr/bin/env bash

protoc -I ./code/go/0chain.net/blobbercore/blobbergrpc/proto --go-grpc_out=. --go_out=. --grpc-gateway_out=. --openapiv2_out=./code/go/0chain.net/blobbercore/openapi ./code/go/0chain.net/blobbercore/blobbergrpc/proto/blobber.proto