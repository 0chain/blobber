#!/bin/sh
PWD=`pwd`
BLOBBER_DIR=`basename $PWD`
BLOBBER_ID=`echo my directory $BLOBBER_DIR | sed -e 's/.*\(.\)$/\1/'`

SSD_PATH="${1:-.}"
HDD_PATH="${2:-.}"

echo Starting blobber$BLOBBER_ID ...

# echo blobber$i

BLOBBER=$BLOBBER_ID SSD_PATH=$PROJECT_ROOT_SSD HDD_PATH=$PROJECT_ROOT_HDD docker-compose -p blobber$BLOBBER_ID -f ../p0docker-compose.yml up -d
