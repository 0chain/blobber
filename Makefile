########################################################
########################################################
########################################################
########################################################

# Scripts Related to developing Blobber

########################################################
########################################################
########################################################
########################################################
UNAME_OS := $(shell uname -s)
UNAME_ARCH := $(shell uname -m)
ROOT := $(shell pwd)

.PHONY: test
test:
	CGO_ENABLED=1 go test -tags bn256  ./...

.PHONY: lint
lint:
	golangci-lint run --timeout 2m0s;


.PHONY: local-init
local-init:
	@echo "=========================[ init blobber ]========================="
	mkdir -p ./dev.local/data/blobber 
	#[ -d ./dev.local/data/blobber/config ] && rm -rf ./dev.local/data/blobber/config
	cp -r ./config ./dev.local/data/blobber/ 
ifeq ($(UNAME_OS),Darwin)
	cd ./dev.local/data/blobber/config/ && find . -name "*.yaml" -exec sed -i '' "s/host: postgres/host: 127.0.0.1/g" {} \;
else
	cd ./dev.local/data/blobber/config/ && sed -i "s/host: postgres/host: 127.0.0.1/g" ./0chain_blobber.yaml
endif
	cd ./dev.local/data/blobber && [ -d files ] || mkdir files 
	cd ./dev.local/data/blobber && [ -d data ] || mkdir data 
	cd ./dev.local/data/blobber && [ -d log ] || mkdir log

.PHONY: local-build
local-build: local-init
	@echo "=========================[ build blobber ]========================="
	cd ./code/go/0chain.net/blobber && CGO_ENABLED=1 go build -tags "bn256 development" -ldflags "-X github.com/0chain/blobber/code/go/0chain.net/core/build.BuildTag=dev" -o ../../../../dev.local/data/blobber/blobber .


.PHONY: local-run
local-run: 
	@echo "=========================[ run blobber ]========================="
	cd ./dev.local/ && ./data/blobber/blobber \
	--port 25051 \
	--grpc_port 35051 \
	--hostname 127.0.0.1 \
	--deployment_mode 0 \
	--keys_file ../docker.local/keys_config/b0bnode1_keys.txt  \
	--files_dir ./data/blobber/files \
	--log_dir ./data/blobber/log \
	--db_dir ./data/blobber/data  \
	--config_dir ../config
    

########################################################
########################################################
########################################################
########################################################

# Scripts related to setting up Buf

########################################################
########################################################
########################################################
########################################################


SHELL := /usr/bin/env bash -o pipefail

# This controls the location of the cache.
PROJECT := blobber
# This controls the remote HTTPS git location to compare against for breaking changes in CI.
#
# Most CI providers only clone the branch under test and to a certain depth, so when
# running buf breaking in CI, it is generally preferable to compare against
# the remote repository directly.
#
# Basic authentication is available, see https://buf.build/docs/inputs#https for more details.
HTTPS_GIT := https://github.com/0chain/blobber.git
# This controls the remote SSH git location to compare against for breaking changes in CI.
#
# CI providers will typically have an SSH key installed as part of your setup for both
# public and private repositories. Buf knows how to look for an SSH key at ~/.ssh/id_rsa
# and a known hosts file at ~/.ssh/known_hosts or /etc/ssh/known_hosts without any further
# configuration. We demo this with CircleCI.
#
# See https://buf.build/docs/inputs#ssh for more details.
SSH_GIT := git@github.com:0chain/blobber.git
# This controls the version of buf to install and use.
BUF_VERSION := 0.44.0
# If true, Buf is installed from source instead of from releases
BUF_INSTALL_FROM_SOURCE := false

### Everything below this line is meant to be static, i.e. only adjust the above variables. ###


# Buf will be cached to ~/.cache/buf-example.
CACHE_BASE := $(HOME)/.cache/$(PROJECT)
# This allows switching between i.e a Docker container and your local setup without overwriting.
CACHE := $(CACHE_BASE)/$(UNAME_OS)/$(UNAME_ARCH)
# The location where buf will be installed.
CACHE_BIN := $(CACHE)/bin
# Marker files are put into this directory to denote the current version of binaries that are installed.
CACHE_VERSIONS := $(CACHE)/versions

# Update the $PATH so we can use buf directly
export PATH := $(abspath $(CACHE_BIN)):$(PATH)
# Update GOBIN to point to CACHE_BIN for source installations
export GOBIN := $(abspath $(CACHE_BIN))
# This is needed to allow versions to be added to Golang modules with go get
export GO111MODULE := on

# BUF points to the marker file for the installed version.
#
# If BUF_VERSION is changed, the binary will be re-downloaded.
BUF := $(CACHE_VERSIONS)/buf/$(BUF_VERSION)
$(BUF):
	@rm -f $(CACHE_BIN)/buf
	@mkdir -p $(CACHE_BIN)
ifeq ($(BUF_INSTALL_FROM_SOURCE),true)
	$(eval BUF_TMP := $(shell mktemp -d))
	cd $(BUF_TMP); go get github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	@rm -rf $(BUF_TMP)
else
	curl -sSL \
		"https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-$(UNAME_OS)-$(UNAME_ARCH)" \
		-o "$(CACHE_BIN)/buf"
	chmod +x "$(CACHE_BIN)/buf"
endif
	@rm -rf $(dir $(BUF))
	@mkdir -p $(dir $(BUF))
	@touch $(BUF)

.DEFAULT_GOAL := local

# deps allows us to install deps without running any checks.

.PHONY: deps
deps: $(BUF)

# local is what we run when testing locally.
# This does breaking change detection against our local git repository.

.PHONY: local
local: $(BUF)
	buf lint

# https is what we run when testing in most CI providers.
# This does breaking change detection against our remote HTTPS git repository.

.PHONY: https
https: $(BUF)
	buf lint
	buf breaking --against "$(HTTPS_GIT)#branch=master"

# ssh is what we run when testing in CI providers that provide ssh public key authentication.
# This does breaking change detection against our remote HTTPS ssh repository.
# This is especially useful for private repositories.

.PHONY: ssh
ssh: $(BUF)
	buf lint
	buf breaking --against "$(SSH_GIT)#branch=master"

# clean deletes any files not checked in and the cache for all platforms.

.PHONY: clean
clean:
	git clean -xdf
	rm -rf $(CACHE_BASE)

# For updating this repository

.PHONY: updateversion
updateversion:
ifndef VERSION
	$(error "VERSION must be set")
else
ifeq ($(UNAME_OS),Darwin)
	sed -i '' "s/BUF_VERSION := [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*/BUF_VERSION := $(VERSION)/g" Makefile
else
	sed -i "s/BUF_VERSION := [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*/BUF_VERSION := $(VERSION)/g" Makefile
endif
endif

.PHONY: markdown-docs
markdown-docs:
	swagger generate spec -o ./swagger.yaml -w ./code/go/0chain.net -m
	sed -i '' "s/in\:\ form/in\:\ formData/g" ./swagger.yaml
	yq -i '(.paths.*.*.parameters.[] | select(.in == "formData") | select(.type == "object")).type = "file"' swagger.yaml
	swagger generate markdown -f ./swagger.yaml --output=swagger.md
