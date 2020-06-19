# Blobber Setup

## Table of Contents

- [Initial Setup](#initial-setup) - [Directory Setup for Blobbers](#directory-setup-for-blobbers)
- [Building and Starting the Blobber](#building-and-starting-the-nodes)
- [Troubleshooting](#troubleshooting)
- [Connect to other network](#connect-to-other-network)
- [Miscellaneous](#miscellaneous) - [Cleanup](#cleanup) - [Minio Setup](#minio)

## Initial Setup

### Directory Setup for Blobbers

In the git/blobber run the following command

```
$ ./docker.local/bin/blobber.init.setup.sh
```

## Building and Starting the Nodes

1. Go to git/blobber directory to build containers using

```
$ ./docker.local/bin/build.blobber.sh
```

2. After building the container for blobber, go to Blobber1 directory (git/blobber/docker.local/blobber1) and run the container using

```
$ ../bin/blobber.start_bls.sh
```

## Troubleshooting

1. Ensure the port mapping is all correct:

```
$ docker ps
```

This should display a container image blobber_blobber and should have the ports mapped like "0.0.0.0:5050->5050/tcp"

## Connect to other network

- Your network connection depends on the block_worker url you give in the `0chain_blobber/validator.yaml` config file.

```
block_worker: http://198.18.0.98:9091
```

This works as a dns service, You need to know the above url for any network you want to connect, Just replace it in the above metioned file.
For example: If you want to connect to one network

```
block_worker: http://one.testnet-0chain.net/dns
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
TIER_BUCKET_NAME
COPY_BUCKET_NAME
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
  # Number of workers to run in parallel, Just to make execution faster we can have mutiple workers running simultaneously
  num_workers: 5
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
