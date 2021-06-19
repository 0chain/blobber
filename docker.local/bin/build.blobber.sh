#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

<<<<<<< HEAD
[ -d ./gosdk ] && rm -rf gosdk
=======
cmd="build"

for arg in "$@"
do
    case $arg in
        -m1|--m1|m1)
        echo "The build will be performed for Apple M1 chip"
        cmd="buildx build --platform linux/amd64"
        shift
        ;;
    esac
done

docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/ValidatorDockerfile . -t validator
docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/Dockerfile . -t blobber
>>>>>>> master

cp -r ../gosdk ./


docker build --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/blobber.Dockerfile . -t blobber

