#!/usr/bin/env bash
echo Starting blobber ...
#docker-compose -p zchain -f /0chain/docker.aws/build.blobber/docker-compose.yml up --detach
docker-compose  -f /0chain/docker.aws/build.blobber/docker-compose.yml up --detach

