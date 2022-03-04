#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

if [ -z "$DOCKER_BUILD" ]; then  
    if [ "x86_64" != "$(uname -m)" ]; then
        docker buildx use blobber_buildx || docker buildx create --name blobber_buildx --use
        DOCKER_BUILD="buildx build --platform linux/arm64"
    else
        DOCKER_BUILD="build"
    fi
fi
echo "1> download herumi"
[ ! -f ./docker.local/bin/mcl-master.tar.gz ] && wget -O ./docker.local/bin/mcl-master.tar.gz https://github.com/herumi/mcl/archive/master.tar.gz 

[ ! -f ./docker.local/bin/bls-master.tar.gz ] && wget -O ./docker.local/bin/bls-master.tar.gz https://github.com/herumi/bls/archive/master.tar.gz 

echo "2> docker build"
docker $DOCKER_BUILD --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/base.Dockerfile . -t blobber_base