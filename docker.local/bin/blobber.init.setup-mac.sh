#!/bin/sh

for i in $(seq 1 6)
do
  mkdir -p docker.local/blobber$i
  mkdir -p docker.local/validator$i
done
