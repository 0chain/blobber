#!/usr/bin/env bash

protoc -I ./blobbergrpc/proto --go-grpc_out=. --go_out=. --grpc-gateway_out=. ./blobbergrpc/proto/blobber.proto