#!/bin/sh
  
for i in $(seq 1 20)
do
  if [[ $i -lt 10 ]]; then
    i=0$i
  fi
  rm -rf docker.local/blobber$i
  rm -rf docker.local/validator$i
done
