#!/bin/bash
cd docker.local/blobber1
../bin/blobber.stop_bls.sh
cd -
cd docker.local/blobber2
../bin/blobber.stop_bls.sh
cd -
cd docker.local/blobber3
../bin/blobber.stop_bls.sh
cd -
cd docker.local/blobber4
../bin/blobber.stop_bls.sh
