#!/bin/bash

set -x

mkdir zus-repos

cd zus-repos


declare -a repo=(

[0]=git@github.com:0chain/0chain.git
[1]=git@github.com:0chain/blobber.git
[2]=git@github.com:0chain/0dns.git

)

for i in ${!repo[@]}; do
  git clone  ${repo[$i]}
done

# setting up 0dns
# echo 'y' | docker system prune -a

cd 0dns

CONFIG_FILE="docker.local/config/0dns.yaml"
sed -i "s/use_https.*/use_https: false/" "$CONFIG_FILE"
sed -i "s/use_path.*/use_path: false/" "$CONFIG_FILE"

echo " docker build"
docker.local/bin/build.sh

# # setting up 0chain
cd ../0chain
# pwd

docker.local/bin/setup.network.sh

# starting 0dns
cd ../0dns
docker.local/bin/start.sh

#  docker container ps

# # starting 0chain
# cd zus-repos/0chain
# pwd
cd ../0chain

docker.local/bin/init.setup.sh

ls docker.local/

docker.local/bin/build.base.sh

install mockery
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
brew doctor
echo "" | brew install mockery

# make build mocks
make build-mocks 

# build sharder
docker.local/bin/build.sharders.sh

# build miner
docker.local/bin/build.miners.sh


cd ../blobber

chmod +x docker.local/bin/blobber.init.setup.sh
docker.local/bin/blobber.init.setup.sh

# # # #cd zus-repos/blobber

ls docker.local/

cd config

CONFIG_FILE="0chain_blobber.yaml"

sed -i 's/block_worker.*/block_worker: http:\/\/198.18.0.98:9091\/dns/' "$CONFIG_FILE"

CONFIG_FILE="0chain_validator.yaml"
# # sed -i "s/block_worker.*/block_worker: http://198.18.0.98:9091/dns/" "$CONFIG_FILE"
sed -i 's/block_worker.*/block_worker: http:\/\/198.18.0.98:9091\/dns/' "$CONFIG_FILE"

cd ..

docker.local/bin/build.base.sh
docker.local/bin/build.blobber.sh
docker.local/bin/build.validator.sh

# cd zus-repos/0chain/docker.local
cd ../0chain/docker.local


for i in {1..2}; 
do
   (
     cd sharder$i/
    ../bin/start.b0sharder.sh
    cd ..
   ) &
   
done


echo " completed first loop"

# running miners 

for i in {1..3}; 
do 
    (
      cd miner$i/
      ../bin/start.b0miner.sh
      cd ..
    ) &
   
done


# making zwallet

cd ../..

sudo apt update

sudo apt-get install build-essential

git clone https://github.com/0chain/zwalletcli.git

cd zwalletcli

make install

./zwallet

if [ -d "$HOME/.zcn" ]; then

   rm -r $HOME/.zcn

fi



mkdir $HOME/.zcn

cp network/config.yaml $HOME/.zcn/config.yaml

CONFIG_FILE="$HOME/.zcn/config.yaml"
# # # sed -i "s/block_worker.*/block_worker: http://198.18.0.98:9091/dns/" "$CONFIG_FILE"
sed -i 's/block_worker.*/block_worker: http:\/\/198.18.0.98:9091\/dns/' "$CONFIG_FILE"

CONFIG_FILE="network/config.yaml"
# # # sed -i "s/block_worker.*/block_worker: http://198.18.0.98:9091/dns/" "$CONFIG_FILE"
sed -i 's/block_worker.*/block_worker: http:\/\/198.18.0.98:9091\/dns/' "$CONFIG_FILE"

# #create wallet

./zwallet create-wallet

./zwallet faucet --methodName pour --input "new wallet"

./zwallet getbalance

# Execute faucet smart contract success



# client_id="$(grep -Po '"client_id": *\K"[^"]*"' $HOME/.zcn/wallet.json)"

# cd ../blobber/config

# CONFIG_FILE="0chain_blobber.yaml"
# sed -i "s/delegate_wallet.*/delegate_wallet: $client_id/" "$CONFIG_FILE"
# CONFIG_FILE="0chain_validator.yaml"
# sed -i "s/delegate_wallet.*/delegate_wallet: $client_id/" "$CONFIG_FILE"

# cd ../..

# touch $HOME/.zcn/network.yaml

# yaml_content="miners:
#   - http://localhost:7071
#   - http://localhost:7072
#   - http://localhost:7073
# sharders:
#   - http://localhost:7171
#   - http://localhost:7172/"

# echo "$yaml_content" >> $HOME/.zcn/network.yaml


# cd blobber/docker.local

# for i in {1..6}; 
# do 
#     (
#       cd blobber$i/
#       ../bin/blobber.start_bls.sh
#       cd ..
#     ) &
   
# done