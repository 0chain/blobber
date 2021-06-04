#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

[ -d ./gosdk ] && rm -rf gosdk

cp -r ../gosdk ./


docker build --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/dev.validator.Dockerfile . -t validator
docker build --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/dev.blobber.Dockerfile . -t blobber

rm -rf ./gosdk

for i in $(seq 1 6);
do
  BLOBBER=$i docker-compose -p blobber$i -f docker.local/dev.docker-compose.yml build --force-rm
done


