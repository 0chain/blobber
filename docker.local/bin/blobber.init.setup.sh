#!/bin/sh

for i in $(seq 1 6)
do
  mkdir -p docker.local/blobber$i
  mkdir /mnt/ssd/blobber$i
  mkdir /mnt/hdd/blobber$i
  mkdir -p docker.local/validator$i
  mkdir /mnt/hdd/validator$i
done
