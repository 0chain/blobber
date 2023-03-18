#!/bin/bash

set -e

echo "1> set DOCKER_IMAGE & DOCKER_BUILD"
# if [ -z "$DOCKER_BUILD" ]; then
#     if [ "x86_64" != "$(uname -m)" ]; then
#         #docker buildx use blobber_buildx || docker buildx create --name blobber_buildx --use
#         DOCKER_BUILD="buildx build --platform linux/arm64"
#     else
#         DOCKER_BUILD="build"
#     fi
# fi

if [ -z "$DOCKER_IMAGE_BASE" ]; then
    DOCKER_IMAGE_BASE="blobber_base"
fi

dockerfile="docker.local/Dockerfile.swagger"
platform=""

DOCKER_BUILDKIT=1 docker $DOCKER_BUILD --progress=plain --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE -f $dockerfile . -t swagger_test

echo "swagger_test docker image is successfully build"
echo "print docker conatiners."
docker ps -a
echo "print docker images"
docker images

docker run $platform $INTERACTIVE -v $(pwd):/codecov  swagger_test bash -c "\
cd /codecov/code/go/0chain.net/; \
swagger generate spec -w . -m -o swagger.yaml; \
swagger generate markdown -f swagger.yaml --output=swagger.md"

echo "print docker conatiners"
docker ps -a
