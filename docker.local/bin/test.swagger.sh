#!/bin/bash

set -e

if [ -z "$DOCKER_IMAGE_BASE" ]; then
    DOCKER_IMAGE_BASE="blobber_base"
fi

cmd="build --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE"

dockerfile="docker.local/Dockerfile.swagger"
platform=""

for arg in "$@"
do
    case $arg in
        -m1|--m1|m1)
        echo "The build will be performed for Apple M1 chip"
        cmd="buildx build --platform linux/amd64"
        dockerfile="docker.local/build.unit_test/Dockerfile.m1"
        platform="--platform=linux/amd64"
        shift
        ;;
    esac
done

docker $cmd -f $dockerfile . -t swagger_test

docker run $platform $INTERACTIVE -v $(pwd):/codecov  swagger_test bash -c "\
cd /codecov/code/go/0chain.net/; \
swagger generate spec -w . -m -o swagger.yaml; \
swagger generate markdown -f swagger.yaml --output=swagger.md
"
