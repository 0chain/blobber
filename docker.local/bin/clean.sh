#!/bin/sh
  
for i in $(seq 1 6)
do
  rm docker.local/blobber$i/log/*
  rm -rf docker.local/blobber$i/data/badgerdb/*
  rm -rf docker.local/blobber$i/files/*
done

