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

docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/ValidatorDockerfile . -t validator
docker $cmd --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/IntegrationTestsBlobberDockerfile . -t blobber

for i in $(seq 1 6);
do
  BLOBBER=$i docker-compose -p blobber$i -f docker.local/b0docker-compose.yml build --force-rm
done

docker.local/bin/sync_clock.sh
