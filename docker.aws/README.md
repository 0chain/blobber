# Blobber Setup on ec2 / vm / bare metal

## Prerequisite

- ec2 / vm / bare metal  with docker installed

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
2. Update block_worker url you give in the `config/0chain_blobber/validator.yaml` and `config/0chain_blobber/0chain_blobber.yaml` config file.

For example: If you want to connect to beta network, set


```

block_worker: https://beta.0chain.net/dns

```
3. Modify `docker.local/keys_config/b0bnode1_keys.txt` and replace `localhost` with public ip of your instance / vm.

4. Modify `docker.local/b0docker-compose.yml` and replace `< public ip here >` with public ip of your instance / vm.


` command: ./bin/blobber --port 505${BLOBBER} --hostname < public ip here > --deployment_mode 0 --keys_file keysconfig/b0bnode${BLOBBER}_keys.txt --files_dir /blobber/files --log_dir /blobber/log --db_dir /blobber/data --minio_file keysconfig/minio_config.txt `
 

5. Go to git/blobber directory to build containers using 
  

```

$ ./docker.local/bin/build.blobber.sh

```
  

6. After building the container for blobber, go to Blobber1 directory (git/blobber/docker.local/blobber1) and run the container using

  

```

$ ../bin/blobber.start_bls.sh


```

This will join blobber1 to the network. You can repeat step 3 and 5 for other blobbers to join them to the network.

