#!/bin/sh

for i in $(seq 1 6)
do
  mkdir -p docker.local/blobber$i/files
  mkdir -p docker.local/blobber$i/data/badgerdb/blobberstate
  mkdir -p docker.local/blobber$i/log	
done
