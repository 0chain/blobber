#!/usr/bin/env bash

protoc -I ./blobbergrpc/proto --go-grpc_out=. --go_out=. --openapiv2_out=./openapi ./blobbergrpc/proto/blobber.proto