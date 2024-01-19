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

if [ -z "$DOCKER_IMAGE_BLOBBER" ]; then
    DOCKER_IMAGE_BLOBBER="-t blobber"
fi

echo "  DOCKER_BUILD=$DOCKER_BUILD"
echo "  DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE"
echo "  DOCKER_IMAGE_SWAGGER=$DOCKER_IMAGE_SWAGGER"
echo "  DOCKER_IMAGE_BLOBBER=$DOCKER_IMAGE_BLOBBER"

echo ""

echo "generating swagger"
docker.local/bin/test.swagger.sh

echo "2> docker build blobber"
# DOCKER_BUILDKIT=1 docker $DOCKER_BUILD --progress=plain --build-arg GIT_COMMIT=$GIT_COMMIT --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE -f docker.local/blobber.Dockerfile . $DOCKER_IMAGE_BLOBBER
docker $DOCKER_BUILD --build-arg GIT_COMMIT=$GIT_COMMIT --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE -f docker.local/blobber.Dockerfile . $DOCKER_IMAGE_BLOBBER
