#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${DEPLOY_REF:?DEPLOY_REF is required}"
: "${DEPLOY_SHA:?DEPLOY_SHA is required}"
: "${WWW_UID:?WWW_UID is required}"
: "${WWW_GID:?WWW_GID is required}"

case "${WWW_UID}:${WWW_GID}" in
  *[!0-9:]*|:*|*:|"")
    echo "WWW_UID/WWW_GID must be numeric" >&2
    exit 1
    ;;
esac

BUILD_TIME="${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
BUILD_CACHE_REF="${BUILD_CACHE_REF:-${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:buildcache}"
CACHE_ARGS="--cache-from type=registry,ref=${BUILD_CACHE_REF} --cache-to type=registry,ref=${BUILD_CACHE_REF},mode=max,ignore-error=true"

set -- --tag "${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:${DEPLOY_SHA}"
case "${PUBLISH_LATEST-true}" in
  true) set -- "$@" --tag "${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:latest" ;;
  false) ;;
  *) echo "PUBLISH_LATEST must be true or false" >&2; exit 1 ;;
esac

# shellcheck disable=SC2086
docker buildx build \
  --file "$DOCKERFILE" \
  --push \
  "$@" \
  --build-arg "VERSION=${DEPLOY_REF}" \
  --build-arg "BUILD_TIME=${BUILD_TIME}" \
  --build-arg "GIT_COMMIT=${DEPLOY_SHA}" \
  --build-arg "RUN_UID=${WWW_UID}" \
  --build-arg "RUN_GID=${WWW_GID}" \
  $CACHE_ARGS \
  .
