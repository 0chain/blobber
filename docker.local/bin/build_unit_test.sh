#!/bin/sh
set -e

docker build -f docker.local/build.unit_test/Dockerfile . -t blobber_unit_test