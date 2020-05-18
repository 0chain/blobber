# Blobber Setup

## Table of Contents

- [Initial Setup](#initial-setup) - [Directory Setup for Blobbers](#directory-setup-for-blobbers)
- [Building and Starting the Blobber](#building-and-starting-the-nodes)
- [Troubleshooting](#troubleshooting)
- [Connect to other network](#connect-to-other-network)
- [Miscellaneous](#miscellaneous) - [Cleanup](#cleanup)

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

- Your network connection depends on the nodes file you give in the docker-compose while starting the blobber node

```
--nodes_file b0single_machine_3_nodes
```

The nodes files are inside the `docker.local/keys_config` foler, You can use any preset file or add/edit as per your requirement.

- To use one network, Use the one.yaml which is already present in the `docker.local/keys_config` folder.

```
--nodes_file one
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
