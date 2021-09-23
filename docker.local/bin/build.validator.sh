#!/bin/sh
set -e

GIT_COMMIT=$(git rev-list -1 HEAD)
echo $GIT_COMMIT

cmd="build"

for arg in "$@"
do
    case $arg in
        -m1|--m1|m1)
        echo "The build will be performed for Apple M1 chip"
        cmd="buildx build --platform linux/amd64"
        shift
        ;;
    esac
done

# [ -d ./gosdk ] && rm -rf gosdk
# cp -r ../gosdk ./

docker $cmd  --build-arg GIT_COMMIT=$GIT_COMMIT -f docker.local/validator.Dockerfile . -t validator

