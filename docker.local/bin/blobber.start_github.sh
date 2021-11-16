#!/bin/bash
set -e

PWD=`pwd`
BLOBBER_DIR=`basename $PWD`
BLOBBER_ID=`echo my directory $BLOBBER_DIR | sed -e 's/.*\(.\)$/\1/'`


BLOBBER=$BLOBBER_ID docker-compose -p blobber$BLOBBER_ID -f ../b0docker-compose-github.yml up -d
