#!/bin/sh

for i in $(seq 0 2);
do
  BLOBBER=$i docker-compose -p blobber$i -f docker.local/docker-compose.yml build --force-rm
done