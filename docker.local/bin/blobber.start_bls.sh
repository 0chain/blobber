#!/bin/bash
set -e

PWD=`pwd`
BLOBBER_DIR=`basename $PWD`
BLOBBER_ID=`echo my directory $BLOBBER_DIR | sed -e 's/.*\(.\)$/\1/'`

if [[ "$*" == *"--debug"* ]]
then
    echo Starting blobber$BLOBBER_ID in debug mode...
    BLOBBER=$BLOBBER_ID docker-compose -p blobber$BLOBBER_ID -f ../b0docker-compose-debug.yml up
else
    echo Starting blobber$BLOBBER_ID ...
    BLOBBER=$BLOBBER_ID docker-compose -p blobber$BLOBBER_ID -f ../b0docker-compose.yml up
fi