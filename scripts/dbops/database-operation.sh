#!/usr/bin/env bash

set -euo pipefail

OPERATION="${IAM_DB_OPS_OPERATION:-}"
BACKUP_NAME="${IAM_DB_OPS_BACKUP_NAME:-}"
BACKUP_DIR="${IAM_DB_OPS_BACKUP_DIR:-/opt/backups/iam/database}"
MYSQL_BIN="${IAM_DB_OPS_MYSQL_BIN:-mysql}"
MYSQLDUMP_BIN="${IAM_DB_OPS_MYSQLDUMP_BIN:-mysqldump}"
TIMESTAMP_OVERRIDE="${IAM_DB_OPS_TIMESTAMP:-}"
ALLOW_DOCKER_CLIENT="${IAM_DB_OPS_ALLOW_DOCKER_CLIENT:-0}"
MYSQL_CLIENT_IMAGE="${IAM_DB_OPS_MYSQL_CLIENT_IMAGE:-mysql:8.0}"

MYSQL_DEFAULTS=""
PARTIAL_PATH=""
ERROR_PATH=""
MYSQL_CLIENT_VERSION=""
CLIENT_WRAPPER_DIR=""

fail() {
  echo "database operation failed: $1" >&2
  return 1
}

cleanup() {
  if [ -n "$MYSQL_DEFAULTS" ]; then
    rm -f -- "$MYSQL_DEFAULTS"
  fi
  if [ -n "$PARTIAL_PATH" ]; then
    rm -f -- "$PARTIAL_PATH"
  fi
  if [ -n "$ERROR_PATH" ]; then
    rm -f -- "$ERROR_PATH"
  fi
  if [ -n "$CLIENT_WRAPPER_DIR" ]; then
    rm -f -- "$CLIENT_WRAPPER_DIR/mysql" "$CLIENT_WRAPPER_DIR/mysqldump"
    rmdir -- "$CLIENT_WRAPPER_DIR" 2>/dev/null || true
  fi
}

prepare_container_client_fallback() {
  local docker_bin sudo_bin wrapper client
  if command -v "$MYSQL_BIN" >/dev/null 2>&1 && command -v "$MYSQLDUMP_BIN" >/dev/null 2>&1; then
    return
  fi
  if [ "$ALLOW_DOCKER_CLIENT" != "1" ]; then
    return
  fi
  if ! [[ "$MYSQL_CLIENT_IMAGE" =~ ^[A-Za-z0-9._/@:-]+$ ]]; then
    fail "MySQL client image is invalid"
    return 1
  fi
  docker_bin="$(command -v "${IAM_DB_OPS_DOCKER_BIN:-docker}" 2>/dev/null || true)"
  sudo_bin="$(command -v "${IAM_DB_OPS_SUDO_BIN:-sudo}" 2>/dev/null || true)"
  if [ -z "$docker_bin" ] || [ -z "$sudo_bin" ] || ! "$sudo_bin" -n "$docker_bin" info >/dev/null 2>&1; then
    fail "containerized MySQL client is unavailable"
    return 1
  fi

  CLIENT_WRAPPER_DIR="$(mktemp -d "${TMPDIR:-/tmp}/iam-mysql-client-wrapper.XXXXXX")"
  chmod 0700 "$CLIENT_WRAPPER_DIR"
  export IAM_DB_OPS_DOCKER_BIN="$docker_bin"
  export IAM_DB_OPS_SUDO_BIN="$sudo_bin"
  export IAM_DB_OPS_MYSQL_CLIENT_IMAGE="$MYSQL_CLIENT_IMAGE"
  for client in mysql mysqldump; do
    wrapper="$CLIENT_WRAPPER_DIR/$client"
    cat >"$wrapper" <<'EOF'
#!/bin/sh
set -eu
client="$(basename "$0")"
defaults=""
for argument in "$@"; do
  case "$argument" in
    --defaults-extra-file=*) defaults="${argument#*=}" ;;
  esac
done
if [ -n "$defaults" ]; then
  exec "$IAM_DB_OPS_SUDO_BIN" -n "$IAM_DB_OPS_DOCKER_BIN" run --rm -i --network host \
    --volume "$defaults:$defaults:ro" "$IAM_DB_OPS_MYSQL_CLIENT_IMAGE" "$client" "$@"
fi
exec "$IAM_DB_OPS_SUDO_BIN" -n "$IAM_DB_OPS_DOCKER_BIN" run --rm -i --network host \
  "$IAM_DB_OPS_MYSQL_CLIENT_IMAGE" "$client" "$@"
EOF
    chmod 0700 "$wrapper"
  done
  if ! command -v "$MYSQL_BIN" >/dev/null 2>&1; then
    MYSQL_BIN="$CLIENT_WRAPPER_DIR/mysql"
  fi
  if ! command -v "$MYSQLDUMP_BIN" >/dev/null 2>&1; then
    MYSQLDUMP_BIN="$CLIENT_WRAPPER_DIR/mysqldump"
  fi
}

trap cleanup EXIT

require_value() {
  local name="$1"
  local value="$2"
  if [ -z "${value//[[:space:]]/}" ]; then
    fail "required configuration is missing ($name)"
    return 1
  fi
  case "$value" in
    *$'\n'*|*$'\r'*)
      fail "required configuration is invalid ($name)"
      return 1
      ;;
  esac
}

validate_configuration() {
  case "$OPERATION" in
    backup|restore|status) ;;
    *) fail "unsupported operation"; return 1 ;;
  esac

  require_value MYSQL_HOST "${MYSQL_HOST:-}" || return 1
  require_value MYSQL_USERNAME "${MYSQL_USERNAME:-}" || return 1
  require_value MYSQL_PASSWORD "${MYSQL_PASSWORD:-}" || return 1
  require_value MYSQL_DBNAME "${MYSQL_DBNAME:-}" || return 1

  if ! [[ "${MYSQL_PORT:-3306}" =~ ^[0-9]+$ ]]; then
    fail "database port is invalid"
    return 1
  fi
  if ! [[ "$MYSQL_DBNAME" =~ ^[A-Za-z0-9_]+$ ]]; then
    fail "database name is invalid"
    return 1
  fi
  if [[ "$BACKUP_DIR" != /* ]] || [ -L "$BACKUP_DIR" ]; then
    fail "backup directory is invalid"
    return 1
  fi
}

require_mysql8_client() {
  local binary="$1"
  local label="$2"
  local version
  if ! command -v "$binary" >/dev/null 2>&1; then
    fail "$label client is unavailable"
    return 1
  fi
  if ! version="$($binary --version 2>/dev/null)" || ! grep -Eq 'Ver 8\.' <<<"$version"; then
    fail "$label client must be MySQL 8.x"
    return 1
  fi
  if [ "$label" = "mysql" ]; then
    MYSQL_CLIENT_VERSION="$(sed -nE 's/.*Ver (8\.[0-9]+(\.[0-9]+)?).*/\1/p' <<<"$version" | head -1)"
    [ -n "$MYSQL_CLIENT_VERSION" ] || MYSQL_CLIENT_VERSION="8.x"
  fi
}

prepare_backup_directory() {
  if [ ! -d "$BACKUP_DIR" ]; then
    sudo mkdir -p -- "$BACKUP_DIR" >/dev/null 2>&1 || {
      fail "backup directory could not be created"
      return 1
    }
    sudo chown "$(id -u):$(id -g)" "$BACKUP_DIR" >/dev/null 2>&1 || {
      fail "backup directory ownership could not be set"
      return 1
    }
  fi
  if [ -L "$BACKUP_DIR" ]; then
    fail "backup directory must not be a symbolic link"
    return 1
  fi
  chmod 0700 "$BACKUP_DIR" >/dev/null 2>&1 || {
    fail "backup directory permissions could not be set"
    return 1
  }
}

prepare_defaults_file() {
  MYSQL_DEFAULTS="$(mktemp "${TMPDIR:-/tmp}/iam-mysql-client.XXXXXX")"
  chmod 0600 "$MYSQL_DEFAULTS"
  cat >"$MYSQL_DEFAULTS" <<EOF
[client]
host=${MYSQL_HOST}
port=${MYSQL_PORT:-3306}
user=${MYSQL_USERNAME}
password=${MYSQL_PASSWORD}
EOF
}

new_backup_timestamp() {
  if [ -n "$TIMESTAMP_OVERRIDE" ]; then
    printf '%s\n' "$TIMESTAMP_OVERRIDE"
    return
  fi
  date +%Y%m%d_%H%M%S
}

backup_database() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  require_mysql8_client "$MYSQLDUMP_BIN" mysqldump || return 1
  command -v gzip >/dev/null 2>&1 || { fail "gzip is unavailable"; return 1; }
  prepare_backup_directory || return 1
  prepare_defaults_file

  local timestamp final_path backup_count backup_size
  timestamp="$(new_backup_timestamp)"
  if ! [[ "$timestamp" =~ ^[0-9]{8}_[0-9]{6}$ ]]; then
    fail "backup timestamp is invalid"
    return 1
  fi
  final_path="$BACKUP_DIR/iam_backup_${timestamp}.sql.gz"
  if [ -e "$final_path" ]; then
    fail "backup destination already exists"
    return 1
  fi
  PARTIAL_PATH="$BACKUP_DIR/.iam_backup_${timestamp}.sql.gz.partial"
  ERROR_PATH="$BACKUP_DIR/.iam_backup_${timestamp}.error"

  echo "database operation started: operation=backup timestamp=$timestamp"
  if ! "$MYSQLDUMP_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" \
    --single-transaction \
    --quick \
    --routines \
    --triggers \
    --events \
    --no-tablespaces \
    --set-gtid-purged=OFF \
    "$MYSQL_DBNAME" 2>"$ERROR_PATH" | gzip -c >"$PARTIAL_PATH"; then
    fail "backup stream did not complete"
    return 1
  fi
  chmod 0600 "$PARTIAL_PATH"
  if [ ! -s "$PARTIAL_PATH" ] || ! gzip -t "$PARTIAL_PATH" 2>/dev/null; then
    fail "backup integrity validation failed"
    return 1
  fi
  mv -- "$PARTIAL_PATH" "$final_path"
  PARTIAL_PATH=""

  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' -print |
    sort -r |
    awk 'NR > 3' |
    while IFS= read -r old; do
      [ -n "$old" ] && rm -f -- "$old"
    done
  backup_count="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' | wc -l | tr -d ' ')"
  backup_size="$(wc -c <"$final_path" | tr -d ' ')"
  echo "database operation completed: operation=backup result=success bytes=$backup_size backups=$backup_count"
}

validate_backup_name() {
  if ! [[ "$BACKUP_NAME" =~ ^iam_backup_[0-9]{8}_[0-9]{6}\.sql\.gz$ ]]; then
    fail "backup name is invalid"
    return 1
  fi
}

restore_database() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  command -v gzip >/dev/null 2>&1 || { fail "gzip is unavailable"; return 1; }
  prepare_backup_directory || return 1
  validate_backup_name || return 1

  local source_path="$BACKUP_DIR/$BACKUP_NAME"
  if [ -L "$source_path" ] || [ ! -f "$source_path" ]; then
    fail "backup file is unavailable"
    return 1
  fi
  if ! gzip -t "$source_path" 2>/dev/null; then
    fail "backup integrity validation failed"
    return 1
  fi
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_restore.error"
  echo "database operation started: operation=restore"
  if ! gzip -dc "$source_path" 2>"$ERROR_PATH" | "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" "$MYSQL_DBNAME" 2>>"$ERROR_PATH"; then
    fail "restore stream did not complete"
    return 1
  fi
  echo "database operation completed: operation=restore result=success"
}

database_status() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  prepare_backup_directory || return 1
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_status.error"

  local database_size table_count backup_count latest_backup
  if ! "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e 'SELECT 1;' > /dev/null 2>"$ERROR_PATH"; then
    fail "database connection failed"
    return 1
  fi
  if ! database_size="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e 'SELECT COALESCE(ROUND(SUM(data_length + index_length) / 1024 / 1024, 2), 0) FROM information_schema.tables WHERE table_schema = DATABASE();' 2>"$ERROR_PATH")"; then
    fail "database metadata query failed"
    return 1
  fi
  if ! table_count="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e 'SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE();' 2>"$ERROR_PATH")"; then
    fail "database metadata query failed"
    return 1
  fi
  backup_count="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' | wc -l | tr -d ' ')"
  latest_backup="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' -print | sort -r | head -1 | sed -E 's/.*iam_backup_([0-9]{8}_[0-9]{6})\.sql\.gz/\1/' || true)"
  [ -n "$latest_backup" ] || latest_backup="none"
  echo "database status: result=success mysql_client=$MYSQL_CLIENT_VERSION connection=success size_mb=$database_size tables=$table_count backups=$backup_count latest_backup=$latest_backup"
}

main() {
  umask 077
  validate_configuration || return 1
  prepare_container_client_fallback || return 1
  case "$OPERATION" in
    backup) backup_database ;;
    restore) restore_database ;;
    status) database_status ;;
  esac
}

main
