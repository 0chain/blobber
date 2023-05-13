
[![Build](https://github.com/0chain/blobber/actions/workflows/build-&-publish-docker-image.yml/badge.svg)](https://github.com/0chain/blobber/actions/workflows/build-&-publish-docker-image.yml)
[![Test](https://github.com/0chain/blobber/actions/workflows/tests.yml/badge.svg)](https://github.com/0chain/blobber/actions/workflows/tests.yml)
[![GoDoc](https://godoc.org/github.com/0chain/blobber?status.png)](https://godoc.org/github.com/0chain/blobber)
[![codecov](https://codecov.io/gh/0chain/blobber/branch/staging/graph/badge.svg)](https://codecov.io/gh/0chain/blobber)

# Blobber Setup
This readme provides instructions on how to run blobber locally

## Table of Contents  

- [Initial Setup](#initial-setup) - [Directory Setup for Blobbers](#directory-setup-for-blobbers)

- [Building and Starting the Blobber](#building-and-starting-the-nodes) 

- [Connect to other network](#connect-to-other-network)

- [Miscellaneous](#miscellaneous) - [Cleanup](#cleanup)

- [Run blobber on ec2 / vm / bare metal](https://github.com/0chain/blobber/blob/master/docker.aws/README.md)

- [Run blobber on ec2 / vm / bare metal over https](https://github.com/0chain/blobber/blob/master/https/README.md)

- [Blobber local development guideline](dev.local/README.md)

## Initial Setup

### Required OS and Software Dependencies

 - Linux (Ubuntu Preferred) Version: 20.04 and Above
 - Mac(Apple Silicon or Intel) Version: Big Sur and Above
 - Windows(Requires WSL ) Version: Windows 11 or 10 version 2004 and above
 - Docker is available for Linux, macOS and Windows platforms. Find instructions for the preferred operating system [here](https://docs.docker.com/engine/install/#supported-platforms).

### Directory Setup for Blobbers

1. Clone the Blobber repository using the command
```
git clone https://github.com/0chain/blobber.git
```
2. In the git/blobber run the following command
  
```
chmod +x ./docker.local/bin/blobber.init.setup.sh
./docker.local/bin/blobber.init.setup.sh
```
**NOTE**: For mac user please run below:
```
chmod +x ./docker.local/bin/blobber.init.setup-mac.sh
./docker.local/bin/blobber.init.setup-mac.sh
```
## Building and Starting the Nodes
  
1. Setup a network called testnet0 for each of these node containers to talk to each other.
 
 ```
docker network create --driver=bridge --subnet=198.18.0.0/15 --gateway=198.18.0.255 testnet0
```
Note: Run all scripts as sudo  

2. Set up the block_worker URL

A block worker URL is a field in the `blobber/config/0chain_validator.yaml` and `blobber/config/0chain_blobber.yaml` configuration files that require the URL of blockchain network you want to connect to. For testing purposes we will connect to the beta 0chain network and replace the default URL in blobber/config/0chain_validator.yaml and 0chain_blobber.yaml with the below-mentioned URL.
```
block_worker: https://demo.zus.network/dns
```

3. Go back to the blobber directory and build blobber containers using the scripts below
```
./docker.local/bin/build.base.sh
./docker.local/bin/build.blobber.sh
./docker.local/bin/build.validator.sh
```
Note: Run all scripts as sudo. 
This would take few minutes.

To link to local gosdk so that the changes are reflected on the blobber build please use the below command(optional)

```
./docker.local/bin/build.blobber.dev.sh

```
For Mac with Apple M1 chip use the following [guide](https://github.com/0chain/blobber/blob/staging/dev.local/README.md) to build and start blobbers.

4. Now create a wallet using zwalletcli and install zboxcli to perform storage operations on blobbers.Instructions for creating wallet and installing zboxcli are available [here](https://github.com/0chain/zboxcli#installation-guides)

5. Once the wallet is created , the wallet information will be stored in wallet.json located in the .zcn folder of the Linux home directory. Now navigate to the .zcn folder (this is created during zbox build) 
```
cd $HOME/.zcn/
```
6. Open the wallet.json file. It should be similar to the similar to the output below:
```
{"client_id":"4af719e1fdb6244159f17922382f162387bae3708250cab6bc1c20cd85fb594c",
"client_key":"da1769bd0203b9c84dc19846ed94155b58d1ffeb3bbe35d38db5bf2fddf5a91c91b22bc7c89dd87e1f1fecbb17ef0db93517dd3886a64274997ea46824d2c119","keys":[{"public_key":"da1769bd0203b9c84dc19846ed94155b58d1ffeb3bbe35d38db5bf2fddf5a91c91b22bc7c89dd87e1f1fecbb17ef0db93517dd3886a64274997ea46824d2c1>
"private_key":"542f6be49108f52203ce75222601397aad32e554451371581ba0eca56b093d19"}],"mnemonics":"butter whisper wheat hope duck mention bird half wedding aim good regret maximum illegal much inch immune unlock resource congress drift>
"version":"1.0","date_created":"2021-09-09T20:22:56+05:30"}
```
7. Copy the client_id value and paste it into blobbers and validators settings. The files can be found in `blobber/config` directory.
  
8. Open both the `blobber/config/0chain_validator.yaml` and `blobber/config/0chain_blobber.yaml` and edit the `delegate_wallet` value with your `client_id` value.

9. Now run the blobbers by navigating into blobber directories for Blobber1 (git/blobber/docker.local/blobber1) and run the container using

```
# For locally build images
../bin/blobber.start_bls.sh

# For remote images
../bin/p0blobber.start.sh

```
**_Note: Replace the localhost form `docker.local/p0docker-compose.yml` to your public IP if you are trying to connect to another network ._**

If you are facing `insufficient balance to pay fee` errors when starting blobbers, you can turn 
off fees in [0chain.yaml.server_chain.smart_contract.miner](https://github.com/0chain/0chain/blob/3c38dfd0920675d86876a5b8895272cb66ded9ad/docker.local/config/0chain.yaml#LL96C3-L96C16)
by adjusting true to false.

## Troubleshooting

10. Ensure the port mapping is all correct:

```
docker ps

```
This should display the container image blobber_blobber and should have the ports mapped like "0.0.0.0:5050->5050/tcp"

11. Now check whether the blobber has registered to the blockchain by running the following zbox command

```
./zbox ls-blobbers
```
In the response you should see the local blobbers mentioned with their urls for example http://198.18.0.91:5051 and http://198.18.0.92:5052

Sample Response:
```
- id:                    0bf5ae461d6474ca1bebba028ea57d646043bbfb6a4188348fd649f0deec5df2
  url:                   http://beta.0chain.net:31304
  used / total capacity: 14.0 GiB / 100.0 GiB
  last_health_check:	  1635347306
  terms:
    read_price:          26.874 mZCN / GB
    write_price:         26.874 mZCN / GB / time_unit
    min_lock_demand:     0.1
    cct:                 2m0s
    max_offer_duration:  744h0m0s
- id:                    7a90e6790bcd3d78422d7a230390edc102870fe58c15472073922024985b1c7d
  url:                   http://198.18.0.92:5052
  used / total capacity: 0 B / 1.0 GiB
  last_health_check:	  1635347427
  terms:
    read_price:          10.000 mZCN / GB
    write_price:         100.000 mZCN / GB / time_unit
    min_lock_demand:     0.1
    cct:                 2m0s
    max_offer_duration:  744h0m0s
- id:                    f65af5d64000c7cd2883f4910eb69086f9d6e6635c744e62afcfab58b938ee25
  url:                   http://198.18.0.91:5051
  used / total capacity: 0 B / 1.0 GiB
  last_health_check:	  1635347950
  terms:
    read_price:          10.000 mZCN / GB
    write_price:         100.000 mZCN / GB / time_unit
    min_lock_demand:     0.1
    cct:                 2m0s
    max_offer_duration:  744h0m0s
- id:                    f8dc4aaf3bb32ae0f4ed575dd6931a42b75e546e07cb37a6e1c6aaf1225891c5
  url:                   http://beta.0chain.net:31305
  used / total capacity: 13.3 GiB / 100.0 GiB
  last_health_check:	  1635347346
  terms:
    read_price:          26.874 mZCN / GB
    write_price:         26.865 mZCN / GB / time_unit
    min_lock_demand:     0.1
    cct:                 2m0s
    max_offer_duration:  744h0m0s
```

Note: When starting multiple blobbers, it could happen that blobbers are not being registered properly (not returned on `zbox ls-blobbers`). 
   
Blobber registration takes some time and adding at least 5 second wait before starting the next blobber usually avoids the issue.
  
12. Now you can create allocations on blobber and store files. 

Note: If unable to create new allocations as shown below.

```
./zbox newallocation --lock 0.5
Error creating allocation: transaction_not_found: Transaction was not found on any of the sharders
```

To fix this issue you must lock some tokens on the blobber.Get the local blobber id using the `./zbox ls-blobbers` and use the following command 

```
zbox sp-lock --blobber_id f65af5d64000c7cd2883f4910eb69086f9d6e6635c744e62afcfab58b938ee25 --tokens 0.5
```

## Connect to other network


- Your network connection depends on the block_worker url you give in the `config/0chain_blobber/validator.yaml` and `0chain_blobber.yaml` config file.
 
```

block_worker: http://198.18.0.98:9091

```

This works as a dns service, You need to know the above url for any network you want to connect, Just replace it in the above mentioned file.

For example: If you want to connect to demo network
  
```

block_worker: https://demo.zus.network/dns

```


## Miscellaneous
 
### Cleanup


1. Get rid of old unused docker resources:

  

```

docker system prune

```

  

2. To get rid of all the docker resources and start afresh:

  

```

docker system prune -a

```

  

3. Stop All Containers

  

```

docker stop $(docker ps -a -q)

```

  

4. Remove All Containers

  

```

docker rm $(docker ps -a -q)

```
