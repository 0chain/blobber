#!/bin/sh

BLOBBER=$BLOBBER_ID docker-compose -p blobber$BLOBBER_ID -f ../docker-compose.yml up