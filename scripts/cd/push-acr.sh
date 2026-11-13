#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=/dev/null
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${DEPLOY_SHA:?DEPLOY_SHA is required}"
: "${ALIYUN_ACR_REGISTRY:?ALIYUN_ACR_REGISTRY is required}"
: "${ALIYUN_ACR_NAMESPACE:?ALIYUN_ACR_NAMESPACE is required}"

SRC="${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:${DEPLOY_SHA}"
DST_BASE="${ALIYUN_ACR_REGISTRY}/${ALIYUN_ACR_NAMESPACE}/${IMAGE_NAME}"

case "${PUBLISH_LATEST-true}" in
  true) set -- "${DEPLOY_SHA}" latest ;;
  false) set -- "${DEPLOY_SHA}" ;;
  *) echo "PUBLISH_LATEST must be true or false" >&2; exit 1 ;;
esac

docker pull "$SRC"
for tag do
  target="${DST_BASE}:$tag"
  docker tag "$SRC" "$target"
  docker push "$target"
  printf 'Image pushed: %s\n' "$target"
done
