#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${DOCKERHUB_USERNAME:?DOCKERHUB_USERNAME is required}"
: "${DEPLOY_SHA:?DEPLOY_SHA is required}"

SOURCE_IMAGE="${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:${DEPLOY_SHA}"

case "${PUBLISH_LATEST-true}" in
  true) set -- "${DEPLOY_SHA}" latest ;;
  false) set -- "${DEPLOY_SHA}" ;;
  *) echo "PUBLISH_LATEST must be true or false" >&2; exit 1 ;;
esac

docker pull "$SOURCE_IMAGE"
for tag do
  target="${DOCKERHUB_USERNAME}/${IMAGE_NAME}:$tag"
  docker tag "$SOURCE_IMAGE" "$target"
  docker push "$target"
  printf 'Image pushed: %s\n' "$target"
done
