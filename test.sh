#!/usr/bin/env bash
set -e

cd code/go/0chain.net; CGO_ENABLED=1 go test ./...;
