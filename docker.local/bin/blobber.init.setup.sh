#!/bin/sh

for i in $(seq 1 6)
do
  if [[ $i -lt 10 ]]; then
    i=0$i
  fi
  mkdir -p docker.local/blobber$i/files
  mkdir -p docker.local/blobber$i/data/postgresql
  mkdir -p docker.local/blobber$i/log
  mkdir -p docker.local/validator$i/data
  mkdir -p docker.local/validator$i/log	
done
