#!/bin/sh
PWD=`pwd`
BLOBBER_DIR=`basename $PWD`
BLOBBER_ID=`echo my directory $BLOBBER_DIR | rev | cut -c -2 | rev`


echo Starting blobber$BLOBBER_ID ...

# echo blobber$i

BLOBBER=$BLOBBER_ID docker-compose -p blobber$BLOBBER_ID -f ../p0docker-compose.yml up -d
