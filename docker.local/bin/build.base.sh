#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT
echo "1> set DOCKER_IMAGE & DOCKER_BUILD"
if [ -z "$DOCKER_BUILD" ]; then  
    if [ "x86_64" != "$(uname -m)" ]; then
        #docker buildx use blobber_buildx || docker buildx create --name blobber_buildx --use
        DOCKER_BUILD="buildx build --platform linux/arm64"
    else
        DOCKER_BUILD="build"
    fi
fi

if [ -z "$DOCKER_IMAGE_BASE" ]; then  
    DOCKER_IMAGE_BASE="blobber_base"
fi
echo "  DOCKER_BUILD=$DOCKER_BUILD"
echo "  DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE"

echo ""
echo "2> download herumi"

[ ! -f ./docker.local/bin/mcl-master.tar.gz ] && wget -O ./docker.local/bin/mcl-master.tar.gz https://github.com/herumi/mcl/archive/master.tar.gz 

[ ! -f ./docker.local/bin/bls-master.tar.gz ] && wget -O ./docker.local/bin/bls-master.tar.gz https://github.com/herumi/bls/archive/master.tar.gz 

echo ""
echo "3> docker build"
DOCKER_BUILDKIT=1 docker $DOCKER_BUILD --progress=plain --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/base.Dockerfile . -t $DOCKER_IMAGE_BASE