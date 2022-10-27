export BLOBBER_REGISTRY=bmanu199/blobber
SHORT_SHA=$(git rev-parse --short HEAD)
export DOCKER_IMAGE_BASE="${BLOBBER_REGISTRY}:base"
# export DOCKER_BUILD="buildx build --platform linux/amd64,linux/arm64 --push"
export DOCKER_IMAGE_BLOBBER="-t ${BLOBBER_REGISTRY}:${SHORT_SHA}"

./docker.local/bin/build.base.sh && ./docker.local/bin/build.blobber.sh
docker push ${BLOBBER_REGISTRY}:${SHORT_SHA}
