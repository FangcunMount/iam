#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=/dev/null
. "$SCRIPT_DIR/image-metadata.sh"

: "${RUNNER_SSH_ALIAS:?RUNNER_SSH_ALIAS is required}"
: "${SERVICE:?SERVICE is required}"
: "${IMAGE_TAG:?IMAGE_TAG is required}"
: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${WWW_UID:?WWW_UID is required}"
: "${WWW_GID:?WWW_GID is required}"

PACKAGE_FILE="${DEPLOY_PACKAGE:-deploy-package-${PACKAGE_SUFFIX}.tar.gz}"
IMAGE_FILE="${DEPLOY_IMAGE_PACKAGE:-deploy-image-${PACKAGE_SUFFIX}.tar.gz}"
REMOTE_PACKAGE="/tmp/deploy-package-${PACKAGE_SUFFIX}.tar.gz"
REMOTE_IMAGE="/tmp/deploy-image-${PACKAGE_SUFFIX}.tar.gz"

for f in "$PACKAGE_FILE" "$IMAGE_FILE"; do
  if [ ! -f "$f" ]; then
    echo "Missing file: $f" >&2
    exit 1
  fi
done

# 把部署所需的环境变量用 printf %q 安全转义后写进 bootstrap 脚本的 export 段。
# 这样变量不会经过 ssh 命令行传递（inline `VAR=val ... bash file` 在含特殊字符的
# secret（如 SUDO_PASSWORD 含空格/#/;）上会被远端 shell 截断，导致脚本被忽略、命令
# 静默成功），从根本上避免"看似成功实则空跑"的问题。
LOCAL_BOOT="$(mktemp)"
trap 'rm -f "$LOCAL_BOOT"' EXIT

emit_export() {
  printf 'export %s=%q\n' "$1" "$2" >>"$LOCAL_BOOT"
}

{
  echo '#!/usr/bin/env bash'
  echo 'set -Eeuo pipefail'
  echo ''
} >"$LOCAL_BOOT"

emit_export SERVICE              "$SERVICE"
emit_export DEPLOY_NODE_HOSTNAME "${DEPLOY_NODE_HOSTNAME:-serverB}"
emit_export DEPLOY_SSH_EXPECTED_HOST "${RUNNER_SSH_HOST:-}"
emit_export IMAGE_TAG            "$IMAGE_TAG"
emit_export DEPLOY_IMAGE_SOURCE  "${DEPLOY_IMAGE_SOURCE:-tarball}"
emit_export IMAGE_TARBALL        "$REMOTE_IMAGE"
emit_export DOCKER_REGISTRY      "$DOCKER_REGISTRY"
emit_export DOCKER_REPOSITORY    "$DOCKER_REPOSITORY"
emit_export ALIYUN_ACR_REGISTRY  "${ALIYUN_ACR_REGISTRY:-}"
emit_export ALIYUN_ACR_NAMESPACE "${ALIYUN_ACR_NAMESPACE:-}"
emit_export GHCR_USERNAME        "${GHCR_USERNAME:-}"
emit_export GITHUB_TOKEN         "${GITHUB_TOKEN:-}"
emit_export DOCKERHUB_USERNAME   "${DOCKERHUB_USERNAME:-}"
emit_export DOCKERHUB_TOKEN      "${DOCKERHUB_TOKEN:-}"
emit_export SUDO_PASSWORD        "${SUDO_PASSWORD:-}"
emit_export WWW_UID              "$WWW_UID"
emit_export WWW_GID              "$WWW_GID"
emit_export PKG_PATH             "$REMOTE_PACKAGE"

cat >>"$LOCAL_BOOT" <<'BOOT'

: "${SERVICE:?SERVICE is required}"
: "${PKG_PATH:?PKG_PATH is required}"

BOOTSTRAP_TMP="/tmp/iam-deploy-bootstrap-${SERVICE}-$$"
mkdir -p "$BOOTSTRAP_TMP"
trap 'rm -rf "$BOOTSTRAP_TMP"' EXIT
tar -xzf "$PKG_PATH" -C "$BOOTSTRAP_TMP"
bash "$BOOTSTRAP_TMP/scripts/cd/remote-deploy.sh"
BOOT

# 脚本含 secret，限制权限
chmod 600 "$LOCAL_BOOT"

# scp 经网络传大文件可能被截断（曾出现 docker load "unexpected EOF"），上传后用
# gzip -t 校验远端文件完整性，损坏则重试，彻底失败则报错退出。
upload_and_verify() {
  local local_file="$1" remote_path="$2" attempts="${3:-3}" i
  for ((i = 1; i <= attempts; i++)); do
    echo "Uploading $(basename "$local_file") -> ${RUNNER_SSH_ALIAS}:${remote_path} (attempt ${i}/${attempts})..."
    if scp "$local_file" "${RUNNER_SSH_ALIAS}:${remote_path}" \
      && ssh "${RUNNER_SSH_ALIAS}" "gzip -t ${remote_path}"; then
      echo "Verified ${remote_path} integrity (gzip -t ok)"
      return 0
    fi
    echo "Upload/verify failed for ${remote_path} (attempt ${i}); retrying..." >&2
    sleep 3
  done
  echo "Failed to upload intact $(basename "$local_file") to ${RUNNER_SSH_ALIAS} after ${attempts} attempts" >&2
  return 1
}

resolved_host="$(ssh -G "${RUNNER_SSH_ALIAS}" 2>/dev/null | awk '$1 == "hostname" { print $2; exit }')"
echo "Upload target SSH HostName: ${resolved_host:-<unknown>} (expected: ${RUNNER_SSH_HOST:-unset})"
if [ -n "${RUNNER_SSH_HOST:-}" ] && [ "${resolved_host:-}" != "$RUNNER_SSH_HOST" ]; then
  echo "FATAL: ${RUNNER_SSH_ALIAS} resolves to ${resolved_host:-<empty>}, expected ${RUNNER_SSH_HOST}" >&2
  echo "Re-run setup-runner-ssh.sh; stale ~/.ssh/config on the runner is the usual cause." >&2
  exit 1
fi

echo "Uploading ${PACKAGE_FILE} and ${IMAGE_FILE} to ${RUNNER_SSH_ALIAS}..."
upload_and_verify "$IMAGE_FILE" "$REMOTE_IMAGE"
upload_and_verify "$PACKAGE_FILE" "$REMOTE_PACKAGE"

REMOTE_BOOT="/tmp/iam-cd-bootstrap-${SERVICE}-$$.sh"
echo "Uploading bootstrap script to ${RUNNER_SSH_ALIAS}:${REMOTE_BOOT} ..."
scp "$LOCAL_BOOT" "${RUNNER_SSH_ALIAS}:${REMOTE_BOOT}"
ssh "${RUNNER_SSH_ALIAS}" "chmod 600 ${REMOTE_BOOT}" || true

# 远端只执行脚本本身，不在命令行 inline 任何变量（变量已写进脚本的 export 段）。
echo "Running remote-deploy.sh on ${RUNNER_SSH_ALIAS}..."
rc=0
ssh "${RUNNER_SSH_ALIAS}" "bash ${REMOTE_BOOT}" || rc=$?
ssh "${RUNNER_SSH_ALIAS}" "rm -f ${REMOTE_BOOT}" || true
if [ "$rc" -ne 0 ]; then
  echo "remote-deploy.sh failed on ${RUNNER_SSH_ALIAS} (exit ${rc})" >&2
  exit "$rc"
fi
