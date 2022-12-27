#!/bin/sh
  
for i in $(seq 1 6)
do
  rm -rf /mnt/hdd/blobber$i
  rm -rf /mnt/ssd/blobber$i
  rm -rf /mnt/hdd/validator$i
  rm -rf docker.local/blobber$i
done
