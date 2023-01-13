#!/bin/sh
  
for i in $(seq 1 6)
do
  sudo rm -rf /mnt/hdd/blobber$i
  sudo rm -rf /mnt/ssd/blobber$i
  sudo rm -rf /mnt/hdd/validator$i
  sudo rm -rf docker.local/blobber$i
  sudo rm -rf docker.local/validator$i
done
