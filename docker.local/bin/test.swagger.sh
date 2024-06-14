#!/bin/bash

set -e

echo "1> set DOCKER_IMAGE & DOCKER_BUILD"
if [ -z "$DOCKER_BUILD" ]; then
    if [ "x86_64" != "$(uname -m)" ]; then
        #docker buildx use blobber_buildx || docker buildx create --name blobber_buildx --use
        DOCKER_BUILD="buildx build --platform linux/amd64,linux/arm64"
    else
        DOCKER_BUILD="build"
    fi
fi

if [ -z "$DOCKER_IMAGE_BASE" ]; then
    DOCKER_IMAGE_BASE="blobber_base"
fi

if [ -z "$DOCKER_IMAGE_SWAGGER" ]; then
    DOCKER_IMAGE_SWAGGER="swagger"
fi

# cmd="build --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE"

dockerfile="docker.local/Dockerfile.swagger"
platform=""
INTERACTIVE=""

# for arg in "$@"
# do
#     case $arg in
#         -m1|--m1|m1)
#         echo "The build will be performed for Apple M1 chip"
#         cmd="buildx build --platform linux/amd64"
#         dockerfile="docker.local/build.unit_test/Dockerfile.m1"
#         platform="--platform=linux/amd64"
#         shift
#         ;;
#     esac
# done

echo "2> Build swagger_test docker image."
DOCKER_BUILDKIT=1 docker $DOCKER_BUILD --progress=plain --build-arg DOCKER_IMAGE_BASE=$DOCKER_IMAGE_BASE -f $dockerfile . -t $DOCKER_IMAGE_SWAGGER --network host

echo "3> Create swagger.yaml file."
docker run $platform $INTERACTIVE -v $(pwd):/codecov $DOCKER_IMAGE_SWAGGER bash -c "\
cd /codecov/code/go/0chain.net/; \
swagger generate spec -w . -m -o swagger.yaml; \
sed -i \"s/in\:\ form/in\:\ formData/g\" ./swagger.yaml; \
yq -i '(.paths.*.*.parameters.[] | select(.in == \"formData\") | select(.type == \"object\")).type = \"file\"' swagger.yaml; \
swagger generate markdown -f swagger.yaml --output=swagger.md"
