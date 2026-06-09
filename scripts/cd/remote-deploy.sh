#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=/dev/null
. "$SCRIPT_DIR/image-metadata.sh"

: "${DOCKER_REGISTRY:?DOCKER_REGISTRY is required}"
: "${DOCKER_REPOSITORY:?DOCKER_REPOSITORY is required}"
: "${GHCR_USERNAME:?GHCR_USERNAME is required}"
: "${WWW_UID:?WWW_UID is required}"
: "${WWW_GID:?WWW_GID is required}"

APP_UID="${WWW_UID}"
APP_GID="${WWW_GID}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

case "$IMAGE_TAG" in
  ""|*[!A-Za-z0-9_.-]*)
    echo "IMAGE_TAG must contain only letters, digits, underscores, periods, and dashes; got: ${IMAGE_TAG}" >&2
    exit 1
    ;;
esac

SUDO_ASKPASS_SCRIPT=""
DEPLOY_TMP="${DEPLOY_TMP:-/tmp/iam-deploy-${PACKAGE_SUFFIX}-$$}"

cleanup() {
  if [ -n "$SUDO_ASKPASS_SCRIPT" ]; then
    rm -f "$SUDO_ASKPASS_SCRIPT"
  fi
  rm -rf "$DEPLOY_TMP"
}
trap cleanup EXIT

if sudo -n true 2>/dev/null; then
  SUDO="sudo"
  echo "Using passwordless sudo."
else
  if [ -z "${SUDO_PASSWORD:-}" ]; then
    echo "sudo needs password. Provide SUDO_PASSWORD or configure NOPASSWD." >&2
    exit 1
  fi
  sudo_pw() { sudo -S "$@" <<<"$SUDO_PASSWORD"; }
  export -f sudo_pw
  SUDO="sudo_pw"
  $SUDO -v || true
  echo "Using sudo with password."

  SUDO_ASKPASS_SCRIPT="$(mktemp)"
  printf '%s\n' '#!/bin/sh' 'printf '\''%s\n'\'' "$SUDO_PASSWORD"' >"$SUDO_ASKPASS_SCRIPT"
  chmod 700 "$SUDO_ASKPASS_SCRIPT"
fi

docker_login_with_token() {
  local username="$1"
  local token="$2"
  local registry="${3:-}"
  local token_file rc

  token_file="$(mktemp)"
  chmod 600 "$token_file"
  printf '%s' "$token" >"$token_file"
  rc=1

  if [ "$SUDO" = "sudo_pw" ]; then
    if [ -n "$registry" ]; then
      if SUDO_ASKPASS="$SUDO_ASKPASS_SCRIPT" sudo -A docker login "$registry" -u "$username" --password-stdin <"$token_file" >/dev/null; then
        rc=0
      else
        rc=$?
      fi
    else
      if SUDO_ASKPASS="$SUDO_ASKPASS_SCRIPT" sudo -A docker login -u "$username" --password-stdin <"$token_file" >/dev/null; then
        rc=0
      else
        rc=$?
      fi
    fi
  else
    if [ -n "$registry" ]; then
      if $SUDO docker login "$registry" -u "$username" --password-stdin <"$token_file" >/dev/null; then
        rc=0
      else
        rc=$?
      fi
    else
      if $SUDO docker login -u "$username" --password-stdin <"$token_file" >/dev/null; then
        rc=0
      else
        rc=$?
      fi
    fi
  fi

  rm -f "$token_file"
  return "$rc"
}

prepare_dirs_and_backup() {
  $SUDO mkdir -p /opt/iam/configs/env /opt/iam/configs/ssl /opt/iam/build/docker /opt/iam/scripts/cd
  $SUDO mkdir -p /data/logs/iam /data/ops/iam-keys

  BACKUP_DIR="/opt/backups/iam/deployments"
  $SUDO mkdir -p "$BACKUP_DIR"
  $SUDO chown "$(id -u):$(id -g)" "$BACKUP_DIR"
  $SUDO chmod 0750 "$BACKUP_DIR"

  local timestamp
  timestamp=$(date +%Y%m%d_%H%M%S)
  if [ -d /opt/iam/configs ] && [ "$(ls -A /opt/iam/configs 2>/dev/null)" != "" ]; then
    $SUDO tar -czf "$BACKUP_DIR/backup_${timestamp}.tar.gz" \
      /opt/iam/configs /data/logs/iam \
      2>/dev/null || echo "No previous version to backup"
  else
    echo "No previous version to backup"
  fi
}

extract_package() {
  if [ ! -f "$PKG_PATH" ]; then
    echo "${PKG_PATH} not found" >&2
    ls -al /tmp/deploy-package*.tar.gz 2>/dev/null || true
    exit 1
  fi

  mkdir -p "$DEPLOY_TMP"
  tar -xzf "$PKG_PATH" -C "$DEPLOY_TMP"
}

sync_package() {
  $SUDO rsync -a "$DEPLOY_TMP/build/docker/docker-compose.prod.yml" /opt/iam/build/docker/docker-compose.prod.yml
  $SUDO rsync -a "$DEPLOY_TMP/configs/" /opt/iam/configs/
  $SUDO rsync -a "$DEPLOY_TMP/scripts/cd/" /opt/iam/scripts/cd/
  $SUDO chown -R "$APP_UID:$APP_GID" /opt/iam/configs
  $SUDO chown -R "$APP_UID:$APP_GID" /data/logs/iam /data/ops/iam-keys
  $SUDO chmod 0750 /data/ops/iam-keys
}

ensure_network() {
  if ! $SUDO docker network ls --format '{{.Name}}' | grep -w infra-network >/dev/null 2>&1; then
    echo "Creating Docker network infra-network (overlay attachable)..."
    $SUDO docker network create --driver overlay --attachable infra-network || {
      echo "Failed to create overlay network. Ensure Docker Swarm is initialized." >&2
      exit 1
    }
  fi

  if ! $SUDO docker network ls --format '{{.Name}}' | grep -w infra-network >/dev/null 2>&1; then
    echo "Required network infra-network not found." >&2
    exit 1
  fi
}

setup_grpc_certs() {
  local grpc_dir="/data/infra/ssl/grpc"
  local grpc_ca="$grpc_dir/ca/ca-chain.crt"
  local grpc_crt="$grpc_dir/server/iam-apiserver.crt"
  local grpc_key="$grpc_dir/server/iam-apiserver.key"
  local f

  $SUDO mkdir -p "$grpc_dir/ca" "$grpc_dir/server"
  $SUDO chmod 0755 "$grpc_dir" "$grpc_dir/ca" "$grpc_dir/server"

  for f in "$grpc_ca" "$grpc_crt" "$grpc_key"; do
    if ! $SUDO test -r "$f"; then
      echo "Missing or unreadable gRPC mTLS file: $f" >&2
      exit 1
    fi
  done

  $SUDO chown "$APP_UID:$APP_GID" "$grpc_ca" "$grpc_crt" "$grpc_key"
  $SUDO chmod 0644 "$grpc_ca" "$grpc_crt"
  $SUDO chmod 0640 "$grpc_key"
  echo "gRPC certs found under $grpc_dir"
}

image_tarball_path() {
  printf '%s' "${IMAGE_TARBALL:-/tmp/deploy-image-${PACKAGE_SUFFIX}.tar.gz}"
}

load_image_from_tarball() {
  local tarball
  tarball="$(image_tarball_path)"
  if [ ! -f "$tarball" ]; then
    return 1
  fi

  echo "Loading ${IMAGE_NAME} from tarball ${tarball}..."
  local load_started load_elapsed
  load_started=$(date +%s)
  # 不能用 `gzip -dc | $SUDO docker load`：带密码 sudo 是 `sudo -S ... <<<"$PASSWORD"`，
  # here-string 会把 docker load 的 stdin 改成密码串、冲掉管道里的镜像数据，导致
  # "unexpected EOF"。改用 docker load -i（自动解压 gzip），stdin 不被占用。
  $SUDO docker load -i "$tarball"
  load_elapsed=$(($(date +%s) - load_started))
  echo "Loaded ${IMAGE_NAME} from tarball in ${load_elapsed}s"
  rm -f "$tarball"
  IMAGE_LOADED_FROM_TARBALL=1
  export IMAGE_LOADED_FROM_TARBALL
  return 0
}

write_compose_env() {
  local compose_registry="$1"
  local compose_image_name="$2"
  local compose_env_file="/opt/iam/.env"
  local local_compose_env

  # docker compose 插值时 shell 环境变量优先级高于 --env-file。bootstrap 已 export
  # DOCKER_REGISTRY=ghcr.io，image-metadata.sh 已 export IMAGE_NAME=iam，若不覆盖，
  # compose 会用这些旧值拼出 ghcr.io/iam:tag（丢掉 repository/namespace 且指向错误
  # registry），导致 tarball 加载的镜像对不上 "No such image"。这里把解析后的值同步
  # export 到 shell，让高优先级的环境变量持有正确的镜像坐标，与 .env 保持一致。
  DOCKER_REGISTRY="$compose_registry"
  IMAGE_NAME="$compose_image_name"
  export DOCKER_REGISTRY IMAGE_NAME

  local_compose_env="$(mktemp)"
  cat > "$local_compose_env" <<EOF
DOCKER_REGISTRY=${compose_registry}
IMAGE_NAME=${compose_image_name}
IMAGE_TAG=${IMAGE_TAG}
WWW_UID=${APP_UID}
WWW_GID=${APP_GID}
EOF
  $SUDO rsync -a "$local_compose_env" "$compose_env_file"
  rm -f "$local_compose_env"
  $SUDO chmod 0640 "$compose_env_file"
  $SUDO chown "$APP_UID:$APP_GID" "$compose_env_file"
  COMPOSE_ENV_FILE="$compose_env_file"
  export COMPOSE_ENV_FILE
}

select_image_and_write_compose_env() {
  local ghcr_login_ok=0
  local compose_registry="$DOCKER_REGISTRY"
  local compose_image_name="${DOCKER_REPOSITORY}/${IMAGE_NAME}"
  echo "Checking registry login for ${DOCKER_REPOSITORY}/${IMAGE_NAME}"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    docker_login_with_token "$GHCR_USERNAME" "$GITHUB_TOKEN" "$DOCKER_REGISTRY" && ghcr_login_ok=1 || ghcr_login_ok=0
  fi

  if [ "$ghcr_login_ok" -ne 1 ]; then
    if [ -n "${DOCKERHUB_USERNAME:-}" ] && [ -n "${DOCKERHUB_TOKEN:-}" ]; then
      echo "GHCR login failed; falling back to Docker Hub..."
      local dockerhub_login_ok=0
      docker_login_with_token "$DOCKERHUB_USERNAME" "$DOCKERHUB_TOKEN" && dockerhub_login_ok=1 || dockerhub_login_ok=0
      if [ "$dockerhub_login_ok" -ne 1 ]; then
        echo "Docker Hub login failed; verify DOCKERHUB_USERNAME/DOCKERHUB_TOKEN." >&2
        exit 1
      fi
      compose_registry="docker.io"
      compose_image_name="${DOCKERHUB_USERNAME}/${IMAGE_NAME}"
    else
      echo "GHCR login failed and Docker Hub credentials are missing; keeping GHCR image."
    fi
  fi

  write_compose_env "$compose_registry" "$compose_image_name"
}

resolve_image_source() {
  local source="${DEPLOY_IMAGE_SOURCE:-auto}"

  case "$source" in
    tarball)
      if ! load_image_from_tarball; then
        echo "DEPLOY_IMAGE_SOURCE=tarball but tarball not found: $(image_tarball_path)" >&2
        exit 1
      fi
      if [ -n "${ALIYUN_ACR_REGISTRY:-}" ] && [ -n "${ALIYUN_ACR_NAMESPACE:-}" ]; then
        write_compose_env "$ALIYUN_ACR_REGISTRY" "${ALIYUN_ACR_NAMESPACE}/${IMAGE_NAME}"
      else
        write_compose_env "$DOCKER_REGISTRY" "${DOCKER_REPOSITORY}/${IMAGE_NAME}"
      fi
      ;;
    auto)
      if load_image_from_tarball; then
        if [ -n "${ALIYUN_ACR_REGISTRY:-}" ] && [ -n "${ALIYUN_ACR_NAMESPACE:-}" ]; then
          write_compose_env "$ALIYUN_ACR_REGISTRY" "${ALIYUN_ACR_NAMESPACE}/${IMAGE_NAME}"
        else
          write_compose_env "$DOCKER_REGISTRY" "${DOCKER_REPOSITORY}/${IMAGE_NAME}"
        fi
        return 0
      fi
      select_image_and_write_compose_env
      ;;
    registry)
      select_image_and_write_compose_env
      ;;
    *)
      echo "DEPLOY_IMAGE_SOURCE must be auto, tarball, or registry; got: ${source}" >&2
      exit 1
      ;;
  esac
}

docker_compose_pull_supports_quiet() {
  $SUDO docker compose pull --help 2>/dev/null | grep -q -- '--quiet'
}

docker_compose() {
  if [ -z "${COMPOSE_ENV_FILE:-}" ] || [ ! -f "$COMPOSE_ENV_FILE" ]; then
    echo "COMPOSE_ENV_FILE is not ready before docker compose execution" >&2
    exit 1
  fi

  (cd /opt/iam && $SUDO docker compose --env-file "$COMPOSE_ENV_FILE" "$@")
}

# 镜像要么已通过 tarball docker load，要么已由 pull 步骤拉好，compose up 不应再回源
# 拉取（否则会因 registry 无凭据而 "error from registry: denied"）。老版本 compose
# 不支持 --pull 时返回空，保持兼容。
compose_up_pull_never_flag() {
  if $SUDO docker compose up --help 2>/dev/null | grep -q -- '--pull'; then
    printf '%s' '--pull never'
  fi
}

deploy_service() {
  local compose_file="/opt/iam/build/docker/docker-compose.prod.yml"
  local pull_started pull_elapsed

  if ! $SUDO docker compose version >/dev/null 2>&1; then
    echo "docker compose is not available on target host." >&2
    exit 1
  fi

  if [ "${IMAGE_LOADED_FROM_TARBALL:-0}" = "1" ]; then
    echo "Image already loaded from tarball; skipping registry pull for ${COMPOSE_SERVICE}"
  else
    echo "Pulling ${COMPOSE_SERVICE} image tag ${IMAGE_TAG}..."
    pull_started=$(date +%s)
    if docker_compose_pull_supports_quiet; then
      docker_compose -f "$compose_file" pull --quiet "$COMPOSE_SERVICE"
    else
      docker_compose -f "$compose_file" pull "$COMPOSE_SERVICE"
    fi
    pull_elapsed=$(($(date +%s) - pull_started))
    echo "Pulled ${COMPOSE_SERVICE} image in ${pull_elapsed}s"
  fi

  # shellcheck disable=SC2046
  docker_compose -f "$compose_file" up -d $(compose_up_pull_never_flag) --force-recreate --remove-orphans "$COMPOSE_SERVICE"
}

verify_service() {
  echo "Waiting for service to be ready..."
  local attempts=0
  local max_attempts=30

  while [ "$attempts" -lt "$max_attempts" ]; do
    if $SUDO docker exec "$CONTAINER_NAME" curl -sf "http://127.0.0.1:${INTERNAL_HTTP_PORT}${HEALTH_PATH}" >/dev/null 2>&1; then
      echo "Health check passed (attempt $attempts)"
      $SUDO docker ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}"
      return 0
    fi

    attempts=$((attempts + 1))
    if [ "$attempts" -lt "$max_attempts" ]; then
      echo "Health check attempt $attempts/$max_attempts failed, retrying in 5 seconds..."
      sleep 5
    fi
  done

  echo "Service failed to start after $max_attempts attempts" >&2
  $SUDO docker ps -a --filter "name=${CONTAINER_NAME}" || true
  $SUDO docker exec "$CONTAINER_NAME" sh -lc '
    echo "--- /etc/resolv.conf ---"
    cat /etc/resolv.conf || true
    echo "--- Redis DNS ---"
    nslookup "$IAM_APISERVER_REDIS_CACHE_HOST" || true
    echo "--- MySQL DNS ---"
    mysql_host="${IAM_APISERVER_MYSQL_HOST%%:*}"
    nslookup "$mysql_host" || true
  ' || true
  $SUDO docker logs --tail 2000 "$CONTAINER_NAME" || true
  exit 1
}

cleanup_old_backups() {
  local old_backups backup_file
  old_backups="$($SUDO ls -t "$BACKUP_DIR"/backup_*.tar.gz 2>/dev/null || true)"
  old_backups="$(printf '%s\n' "$old_backups" | tail -n +11 || true)"
  if [ -n "$old_backups" ]; then
    printf '%s\n' "$old_backups" | while IFS= read -r backup_file; do
      [ -z "$backup_file" ] && continue
      $SUDO rm -f "$backup_file" || true
    done
  fi
}

assert_deploy_hostname() {
  local expected="${DEPLOY_NODE_HOSTNAME:-serverB}"
  local actual
  actual="$(hostname -s 2>/dev/null || hostname)"
  echo "Deploy target hostname: ${actual} (expected: ${expected})"
  if [ "$actual" != "$expected" ]; then
    echo "Refusing deploy on ${actual}; IAM production must run on ${expected}." >&2
    echo "Check organization variable SVRB_HOST points to serverB, not serverA." >&2
    exit 1
  fi
}

echo "=========================================="
echo "Deploying ${CONTAINER_NAME}"
echo "Image tag: ${IMAGE_TAG}"
echo "=========================================="

assert_deploy_hostname
prepare_dirs_and_backup
extract_package
sync_package
ensure_network
setup_grpc_certs
resolve_image_source
deploy_service
verify_service
cleanup_old_backups
rm -f "$PKG_PATH"

echo "=========================================="
echo "${CONTAINER_NAME} deployment completed"
echo "=========================================="
