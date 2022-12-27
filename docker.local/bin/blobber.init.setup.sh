#!/bin/sh

for i in $(seq 1 6)
do
  mkdir -p docker.local/blobber$i
  mkdir -p /mnt/ssd/blobber$i
  mkdir -p /mnt/hdd/blobber$i
  mkdir -p docker.local/validator$i
  mkdir -p /mnt/hdd/validator$i
done
