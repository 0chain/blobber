#!/bin/bash

set -e

# bash script to setup blobbers

#TODO: Fix docker installation
# sudo apt update
sudo apt install -y build-essential jq wget
# install go
GO_VERSION=1.18.5
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH

export NETWORK=demo

# download docker-compose
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version


#### ---- Start Blobber Setup ----- ####

# clone blobbers
git clone https://github.com/0chain/blobber.git
cd blobber
./docker.local/bin/blobber.init.setup.sh

# create docker network
DOCKER_NETWORK="testnet0"
docker network ls | egrep "testnet0" > /dev/null
if [ $? -eq 0 ]; then
    echo "$DOCKER_NETWORK network already exists. Deleting it"
    docker network rm $DOCKER_NETWORK
fi
docker network create --driver=bridge --subnet=198.18.0.0/15 --gateway=198.18.0.255 testnet0

# replace the block_worker URL
sed -i "s,block_worker: http://198.18.0.98:9091,block_worker: http://${NETWORK}.0chain.net/dns," config/0chain_validator.yaml
sed -i "s,block_worker: http://198.18.0.98:9091,block_worker: http://${NETWORK}.0chain.net/dns," config/0chain_blobber.yaml


PUBLIC_IP=$(curl api.ipify.org)

# sed -i "s,198.18.0.6\${BLOBBER},$PUBLIC_IP," docker.local/b0docker-compose.yml
# sed -i "s,198.18.0.9\${BLOBBER},$PUBLIC_IP," docker.local/b0docker-compose.yml
sed -i "s,localhost,$PUBLIC_IP," docker.local/keys_config/b0bnode1_keys.txt

# build blobbers
./docker.local/bin/build.base.sh
./docker.local/bin/build.blobber.sh
./docker.local/bin/build.validator.sh

# setup zbox cli
cd ..

git clone https://github.com/0chain/zboxcli.git
cd zboxcli
make install

mkdir -p $HOME/.zcn
cp network/one.yaml $HOME/.zcn/config.yaml
sed -i "s,block_worker: https://one.devnet-0chain.net/dns,block_worker: http://${NETWORK}.0chain.net/dns," $HOME/.zcn/config.yaml

./zbox register

# go back to blobber folder
cd ../blobber

CLIENT_ID=$(cat $HOME/.zcn/wallet.json | jq .client_id | tr -d '"')
sed -i "s,.*delegate_wallet.*,delegate_wallet: ${CLIENT_ID}," config/0chain_validator.yaml
sed -i "s,.*delegate_wallet.*,delegate_wallet: ${CLIENT_ID}," config/0chain_blobber.yaml
cd docker.local/blobber2
../bin/blobber.start_bls.sh
