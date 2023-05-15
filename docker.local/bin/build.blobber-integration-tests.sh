#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

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

docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/IntegrationTestsValidator.Dockerfile . -t validator --network host
docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/IntegrationTestsBlobber.Dockerfile . -t blobber --network host

for i in $(seq 1 6);
do
  BLOBBER=$i docker-compose -p blobber$i -f docker.local/docker-compose.yml build --force-rm
done

docker.local/bin/sync_clock.sh
