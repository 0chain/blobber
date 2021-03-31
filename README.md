
# Blobber Setup

  

## Table of Contents

  

- [Initial Setup](#initial-setup) - [Directory Setup for Blobbers](#directory-setup-for-blobbers)

- [Building and Starting the Blobber](#building-and-starting-the-nodes) 

- [Troubleshooting](#troubleshooting)

- [Connect to other network](#connect-to-other-network)

- [Miscellaneous](#miscellaneous) - [Cleanup](#cleanup) - [Minio Setup](#minio)

- [Run blobber on ec2 / vm / bare metal](https://github.com/0chain/blobber/blob/master/docker.aws/README.md)

- [Run blobber on ec2 / vm / bare metal over https](https://github.com/0chain/blobber/blob/master/https/README.md)

## Initial Setup

  

### Directory Setup for Blobbers

  

In the git/blobber run the following command

  

```

$ ./docker.local/bin/blobber.init.setup.sh

```

  

## Building and Starting the Nodes

  
1. Setup a network called testnet0 for each of these node containers to talk to each other.
 
 ```

$ docker network create --driver=bridge --subnet=198.18.0.0/15 --gateway=198.18.0.255 testnet0

```
2. Go to git/blobber directory to build containers using (skip this step if you want to use existing docker images)

  

```

$ ./docker.local/bin/build.blobber.sh

```
  

3. After building the container for blobber, name the blobber by creating a directory with the given name eg. blobber1: (git/blobber/docker.local/blobber1). The blobber name must end in an integer. Then cd into this directory and run the container using

  

```
# For locally build images
$ ../bin/blobber.start_bls.sh

# For remote images
$ ../bin/p0blobber.start.sh

```
**_Note: Replace the localhost form `docker.local/p0docker-compose.yml` to your public IP if you are trying to connect to another network ._**

## Troubleshooting

  

1. Ensure the port mapping is all correct:

  

```

$ docker ps

```

  

This should display a container image blobber_blobber and should have the ports mapped like "0.0.0.0:5050->5050/tcp"

2. When starting multiple blobbers, it could happen that blobbers are not being registered properly (not returned on `zbox ls-blobbers`). 
   
Blobber registration takes sometime and adding at least 5 second wait before starting the next blobber usually avoids the issue.
  
3. If unable to create new allocations as shown below.

zbox example

```
zbox newallocation --lock 0.5
Error creating allocation: transaction_not_found: Transaction was not found on any of the sharders
```

To fix this issue, the `delegate_wallet` configured on `config/0chain_blober.yaml` and `config/0chain_validator.yaml` must lock some tokens on the blobber. 
Through zbox, it can be done like the example below.

```
zbox sp-lock --blobber_id f65af5d64000c7cd2883f4910eb69086f9d6e6635c744e62afcfab58b938ee25 --tokens 0.5
```

    

## Connect to other network

  

- Your network connection depends on the block_worker url you give in the `config/0chain_blobber/validator.yaml` and `0chain_blobber.yaml` config file.

  

```

block_worker: http://198.18.0.98:9091

```

  

This works as a dns service, You need to know the above url for any network you want to connect, Just replace it in the above metioned file.

For example: If you want to connect to one network

  

```

block_worker: http://one.devnet-0chain.net/dns

```

  

## Miscellaneous

 
### Cleanup

  

1. Get rid of old unused docker resources:

  

```

$ docker system prune

```

  

2. To get rid of all the docker resources and start afresh:

  

```

$ docker system prune -a

```

  

3. Stop All Containers

  

```

docker stop $(docker ps -a -q)

```

  

4. Remove All Containers

  

```

docker rm $(docker ps -a -q)

```

  

### Minio

  

- You can use the inbuild minio support to store old data on cloud

  

You have to update minio_config file with the cloud creds data, The file can found at `docker.local/keys_config/minio_config.txt`.

The following order is used for the content :

  

```

CONNECTION_URL

ACCESS_KEY_ID

SECRET_ACCESS_KEY

BUCKET_NAME

REGION

```

  

- Your minio config file is then used in the docker-compose while starting the sharder node

  

```

--minio_file keysconfig/minio_config.txt

```

  

- You can either update the setting in the same file which is given above or create a new one with you config and use that as

  

```

--minio_file keysconfig/your_new_minio_config_file.txt

```

  

\*\*\_Note: Do not forget to put the file in the same config folder OR mount your new folder.

  

- Apart from private connection config, There are other options as well in the 0chain_blobber.yaml file to manage minio settings.

  

Sample config

  

```

minio:

# Enable or disable minio backup service

start: false

# The frequency at which the worker should look for files, Ex: 3600 means it will run every 3600 seconds

worker_frequency: 3600 # In Seconds

# Use SSL for connection or not

use_ssl: false

```

  

- You can also tweak the cold storage setting depending how you want to decide which data to move to the cloud.

  

Sample config

  

```

cold_storage:

# Minimum file size to be considered for moving to cloud

min_file_size: 1048576 #in bytes

# Minimum time for which file is not updated or not used

file_time_limit_in_hours: 720 #in hours

# Number of files to be queried and processed at once

job_query_limit: 100

# Capacity in percentage after which the cloud backup should start work

max_capacity_percentage: 50

# Delete local copy once the file is moved to cloud

delete_local_copy: true

# Delete cloud copy if the file is deleted from the blobber by user/other process

delete_cloud_copy: true

```
