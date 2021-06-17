#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT


docker build --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/base.Dockerfile . -t blobber_base