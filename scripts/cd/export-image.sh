#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=/dev/null
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${DEPLOY_SHA:?DEPLOY_SHA is required}"

image_for_registry() {
  registry=$1
  case "$registry" in
    ghcr)
      printf '%s\n' "${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}/${IMAGE_NAME}:${DEPLOY_SHA}"
      ;;
    dockerhub)
      : "${DOCKERHUB_USERNAME:?DOCKERHUB_USERNAME is required for registry dockerhub}"
      printf '%s\n' "${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${DEPLOY_SHA}"
      ;;
    acr)
      : "${ALIYUN_ACR_REGISTRY:?ALIYUN_ACR_REGISTRY is required for registry acr}"
      : "${ALIYUN_ACR_NAMESPACE:?ALIYUN_ACR_NAMESPACE is required for registry acr}"
      printf '%s\n' "${ALIYUN_ACR_REGISTRY}/${ALIYUN_ACR_NAMESPACE}/${IMAGE_NAME}:${DEPLOY_SHA}"
      ;;
    *)
      echo "registry must be ghcr, dockerhub, or acr; got: ${registry}" >&2
      return 1
      ;;
  esac
}

EXPORT_IMAGE_REGISTRY="${EXPORT_IMAGE_REGISTRY:-acr}"
EXPORT_IMAGE_FALLBACK_REGISTRY="${EXPORT_IMAGE_FALLBACK_REGISTRY:-}"
IMAGE=$(image_for_registry "$EXPORT_IMAGE_REGISTRY")

OUTPUT="${DEPLOY_IMAGE_PACKAGE:-deploy-image-${PACKAGE_SUFFIX}.tar.gz}"
PULL_MAX_ATTEMPTS="${PULL_MAX_ATTEMPTS:-4}"
PULL_RETRY_INITIAL_DELAY_SECONDS="${PULL_RETRY_INITIAL_DELAY_SECONDS:-5}"

case "$PULL_MAX_ATTEMPTS" in
  ''|*[!0-9]*|0)
    echo "PULL_MAX_ATTEMPTS must be a positive integer; got: ${PULL_MAX_ATTEMPTS}" >&2
    exit 1
    ;;
esac
case "$PULL_RETRY_INITIAL_DELAY_SECONDS" in
  ''|*[!0-9]*)
    echo "PULL_RETRY_INITIAL_DELAY_SECONDS must be a non-negative integer; got: ${PULL_RETRY_INITIAL_DELAY_SECONDS}" >&2
    exit 1
    ;;
esac

pull_image() {
  image=$1
  registry=$2
  echo "Pulling ${image} (${registry}) for tarball export..."
  pull_started=$(date +%s)
  # Mac mini runner 为 ARM64，目标机为 linux/amd64，必须指定平台
  pull_attempt=1
  pull_retry_delay="$PULL_RETRY_INITIAL_DELAY_SECONDS"
  while :; do
    if docker pull --platform linux/amd64 "$image"; then
      pull_elapsed=$(($(date +%s) - pull_started))
      echo "Pulled ${image} in ${pull_elapsed}s after ${pull_attempt} attempt(s)"
      return 0
    fi
    if [ "$pull_attempt" -ge "$PULL_MAX_ATTEMPTS" ]; then
      echo "Pull failed after ${PULL_MAX_ATTEMPTS} attempts: ${image}" >&2
      return 1
    fi
    echo "Pull attempt ${pull_attempt}/${PULL_MAX_ATTEMPTS} failed; retrying in ${pull_retry_delay}s..." >&2
    sleep "$pull_retry_delay"
    pull_attempt=$((pull_attempt + 1))
    pull_retry_delay=$((pull_retry_delay * 2))
  done
}

if ! pull_image "$IMAGE" "$EXPORT_IMAGE_REGISTRY"; then
  if [ -z "$EXPORT_IMAGE_FALLBACK_REGISTRY" ]; then
    exit 1
  fi
  if [ "$EXPORT_IMAGE_FALLBACK_REGISTRY" = "$EXPORT_IMAGE_REGISTRY" ]; then
    echo "Fallback registry must differ from primary registry: ${EXPORT_IMAGE_REGISTRY}" >&2
    exit 1
  fi
  fallback_image=$(image_for_registry "$EXPORT_IMAGE_FALLBACK_REGISTRY")
  echo "Primary registry ${EXPORT_IMAGE_REGISTRY} unavailable; falling back to ${EXPORT_IMAGE_FALLBACK_REGISTRY}." >&2
  if ! pull_image "$fallback_image" "$EXPORT_IMAGE_FALLBACK_REGISTRY"; then
    echo "Both primary and fallback registry pulls failed." >&2
    exit 1
  fi
  IMAGE=$fallback_image
fi

echo "Exporting ${IMAGE} to ${OUTPUT}..."
export_started=$(date +%s)
# 不用 `docker save | gzip` 管道：sh 无 pipefail，docker save 中途失败时 gzip 仍会把
# 残缺内容压成合法 gzip 并 exit 0，生成"gzip 完整但内含 tar 截断"的坏包（曾导致目标机
# docker load "unexpected EOF"）。改为先 save 到文件（失败即 set -e 退出），再压缩。
RAW_TAR="${OUTPUT%.gz}"
[ "$RAW_TAR" = "$OUTPUT" ] && RAW_TAR="${OUTPUT}.raw.tar"
rm -f "$RAW_TAR"
docker save "$IMAGE" -o "$RAW_TAR"
gzip -1 -c "$RAW_TAR" >"$OUTPUT"
rm -f "$RAW_TAR"
# 端到端自检：确保 gzip 内的 tar 可完整解出，否则在 runner 端立刻失败（避免坏包流向线上）。
if ! gzip -dc "$OUTPUT" | tar -tf - >/dev/null 2>&1; then
  echo "Export integrity check failed: ${OUTPUT} contains a truncated/corrupt tar" >&2
  exit 1
fi
export_elapsed=$(($(date +%s) - export_started))
size="$(du -h "$OUTPUT" | awk '{print $1}')"
echo "Created ${OUTPUT} (${size}) in ${export_elapsed}s"
