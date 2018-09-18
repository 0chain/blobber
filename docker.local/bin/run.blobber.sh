#!/bin/sh

BLOBBER=$i docker-compose -p blobber$i -f ../docker-compose.yml exec $SERVICE $CMD $*