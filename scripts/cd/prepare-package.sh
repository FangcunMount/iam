#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=/dev/null
. "$SCRIPT_DIR/image-metadata.sh"

require_env() {
  local missing=0
  local var

  for var in "$@"; do
    if [ -z "${!var:-}" ]; then
      echo "Missing required env: $var" >&2
      missing=1
    fi
  done

  if [ "$missing" -ne 0 ]; then
    exit 1
  fi
}

default_redis_ssl() {
  REDIS_PORT="${REDIS_PORT:-6379}"
  REDIS_DB="${REDIS_DB:-0}"
  if [ -z "${REDIS_USE_SSL:-}" ]; then
    if [ "$REDIS_PORT" = "6380" ]; then
      REDIS_USE_SSL=true
    else
      REDIS_USE_SSL=false
    fi
  fi
  REDIS_SSL_INSECURE_SKIP_VERIFY="${REDIS_SSL_INSECURE_SKIP_VERIFY:-false}"
}

redact_env_file() {
  sed \
    -e 's/PASSWORD=.*/PASSWORD=***REDACTED***/g' \
    -e 's/ENCRYPTION_KEY=.*/ENCRYPTION_KEY=***REDACTED***/g' \
    "$1"
}

MYSQL_PORT="${MYSQL_PORT:-3306}"
NSQ_LOOKUPD_PORT="${NSQ_LOOKUPD_PORT:-4161}"
NSQ_NSQD_PORT="${NSQ_NSQD_PORT:-4150}"
default_redis_ssl

require_env \
  MYSQL_HOST MYSQL_PORT MYSQL_USERNAME MYSQL_PASSWORD MYSQL_DBNAME \
  REDIS_HOST REDIS_PORT IPD_ENCRYPTION_KEY

if [ -n "${NSQ_LOOKUPD_HOST:-}" ]; then
  NSQ_ENABLED=true
else
  NSQ_ENABLED=false
fi

PACKAGE_DIR="${DEPLOY_PACKAGE_DIR:-deploy-package}"
ENV_FILE="${PACKAGE_DIR}/configs/env/config.prod.env"

rm -rf "$PACKAGE_DIR" "$DEPLOY_PACKAGE"
mkdir -p "${PACKAGE_DIR}/configs/env" "${PACKAGE_DIR}/build/docker" "${PACKAGE_DIR}/scripts/cd"
cp -r configs "$PACKAGE_DIR/"
cp build/docker/docker-compose.prod.yml "${PACKAGE_DIR}/build/docker/docker-compose.prod.yml"
cp scripts/cd/image-metadata.sh scripts/cd/remote-deploy.sh "${PACKAGE_DIR}/scripts/cd/"

cat > "$ENV_FILE" <<EOF
# Auto-generated production environment configuration
IAM_APISERVER_MYSQL_HOST=${MYSQL_HOST}:${MYSQL_PORT}
IAM_APISERVER_MYSQL_USERNAME=${MYSQL_USERNAME}
IAM_APISERVER_MYSQL_PASSWORD=${MYSQL_PASSWORD}
IAM_APISERVER_MYSQL_DATABASE=${MYSQL_DBNAME}

# Redis
IAM_APISERVER_REDIS_CACHE_HOST=${REDIS_HOST}
IAM_APISERVER_REDIS_CACHE_PORT=${REDIS_PORT}
IAM_APISERVER_REDIS_CACHE_USERNAME=${REDIS_USERNAME:-}
IAM_APISERVER_REDIS_CACHE_PASSWORD=${REDIS_PASSWORD:-}
IAM_APISERVER_REDIS_CACHE_DATABASE=${REDIS_DB}
IAM_APISERVER_REDIS_CACHE_USE_SSL=${REDIS_USE_SSL}
IAM_APISERVER_REDIS_CACHE_SSL_INSECURE_SKIP_VERIFY=${REDIS_SSL_INSECURE_SKIP_VERIFY}

IAM_APISERVER_IDP_ENCRYPTION_KEY=${IPD_ENCRYPTION_KEY}

# NSQ configuration
IAM_APISERVER_NSQ_ENABLED=${NSQ_ENABLED}
IAM_APISERVER_NSQ_LOOKUPD_ADDRS=${NSQ_LOOKUPD_HOST:-}:${NSQ_LOOKUPD_PORT}
IAM_APISERVER_NSQ_NSQD_ADDR=${NSQ_NSQD_HOST:-}:${NSQ_NSQD_PORT}
EOF

echo "Generated config.prod.env for ${SERVICE}:"
redact_env_file "$ENV_FILE"

tar -czf "$DEPLOY_PACKAGE" -C "$PACKAGE_DIR" .
echo "Created ${DEPLOY_PACKAGE}"
