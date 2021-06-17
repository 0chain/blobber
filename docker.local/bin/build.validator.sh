#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

[ -d ./gosdk ] && rm -rf gosdk

cp -r ../gosdk ./


docker build --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/blobber.Dockerfile . -t blobber

