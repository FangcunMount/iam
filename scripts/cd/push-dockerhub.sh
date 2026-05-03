#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${DOCKERHUB_USERNAME:?DOCKERHUB_USERNAME is required}"
: "${DEPLOY_SHA:?DEPLOY_SHA is required}"

SOURCE_IMAGE="${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:${DEPLOY_SHA}"

docker pull "$SOURCE_IMAGE"
docker tag "$SOURCE_IMAGE" "${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest"
docker tag "$SOURCE_IMAGE" "${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${DEPLOY_SHA}"
docker push "${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest"
docker push "${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${DEPLOY_SHA}"

echo "Image pushed to Docker Hub:"
echo "  - ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest"
echo "  - ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${DEPLOY_SHA}"
