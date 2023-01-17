#!/bin/sh

for i in $(seq 1 6)
do
  sudo mkdir -p docker.local/blobber$i
  sudo mkdir -p /mnt/ssd/blobber$i
  sudo mkdir -p /mnt/hdd/blobber$i
  sudo mkdir -p docker.local/validator$i
  sudo mkdir -p /mnt/hdd/validator$i
done
