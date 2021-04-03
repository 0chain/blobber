#!/bin/sh


# Runs each unit test Returns 0 if all of the tests pass
# and 1 if any one of the tests fail.
# Should be updated as more unit tests are written.

docker build -f docker.local/build.unit_test/Dockerfile . -t zchain_unit_test

docker run zchain_unit_test sh -c '
  echo running unit tests
  (cd 0chain.net/core; go test -tags bn256 0chain.net/core/...)
  if [ $? -ne 0 ]; then
    exit 1
  fi
  (cd 0chain.net/validatorcore; go test -tags bn256 0chain.net/validatorcore/...)
  if [ $? -ne 0 ]; then
    exit 1
  fi
  (cd 0chain.net/conductor; go test -tags bn256 0chain.net/conductor/...)
  if [ $? -ne 0 ]; then
    exit 1
  fi
  (cd 0chain.net/blobbercore; go test -tags bn256 0chain.net/blobbercore/...)
  if [ $? -ne 0 ]; then
    exit 1
  fi
  exit 0
  '
if [ $? -ne 0 ]
  then exit 1;
fi
