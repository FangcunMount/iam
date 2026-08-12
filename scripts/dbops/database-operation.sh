#!/usr/bin/env bash

set -euo pipefail

OPERATION="${IAM_DB_OPS_OPERATION:-}"
BACKUP_NAME="${IAM_DB_OPS_BACKUP_NAME:-}"
BACKUP_DIR="${IAM_DB_OPS_BACKUP_DIR:-/opt/backups/iam/database}"
CONFIRMATION="${IAM_DB_OPS_CONFIRMATION:-}"
MAX_BACKUP_AGE_SECONDS="${IAM_DB_OPS_MAX_BACKUP_AGE_SECONDS:-7200}"
MYSQL_BIN="${IAM_DB_OPS_MYSQL_BIN:-mysql}"
MYSQLDUMP_BIN="${IAM_DB_OPS_MYSQLDUMP_BIN:-mysqldump}"
TIMESTAMP_OVERRIDE="${IAM_DB_OPS_TIMESTAMP:-}"
ALLOW_DOCKER_CLIENT="${IAM_DB_OPS_ALLOW_DOCKER_CLIENT:-0}"
MYSQL_CLIENT_IMAGE="${IAM_DB_OPS_MYSQL_CLIENT_IMAGE:-mysql:8.0}"
AUTHN_CONTAINER="${IAM_DB_OPS_AUTHN_CONTAINER:-iam-apiserver}"
AUTHN_BATCH_SIZE="${IAM_DB_OPS_AUTHN_BATCH_SIZE:-5000}"
AUTHN_EVIDENCE_FILE="${IAM_DB_OPS_AUTHN_EVIDENCE_FILE:-}"

MYSQL_DEFAULTS=""
PARTIAL_PATH=""
ERROR_PATH=""
MYSQL_CLIENT_VERSION=""
CLIENT_WRAPPER_DIR=""
SQL_PATH=""

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
  if [ -n "$SQL_PATH" ]; then
    rm -f -- "$SQL_PATH"
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
needs_stdin=0
has_execute=0
for argument in "$@"; do
  case "$argument" in
    --defaults-extra-file=*) defaults="${argument#*=}" ;;
    -e|--execute|--execute=*) has_execute=1 ;;
  esac
done
if [ "$client" = "mysql" ] && [ "$has_execute" -eq 0 ] && [ "${1:-}" != "--version" ]; then
  needs_stdin=1
fi
if [ -n "$defaults" ]; then
  if [ "$needs_stdin" -eq 1 ]; then
    exec "$IAM_DB_OPS_SUDO_BIN" -n "$IAM_DB_OPS_DOCKER_BIN" run --rm -i --network host \
      --volume "$defaults:$defaults:ro" "$IAM_DB_OPS_MYSQL_CLIENT_IMAGE" "$client" "$@"
  fi
  exec "$IAM_DB_OPS_SUDO_BIN" -n "$IAM_DB_OPS_DOCKER_BIN" run --rm --network host \
    --volume "$defaults:$defaults:ro" "$IAM_DB_OPS_MYSQL_CLIENT_IMAGE" "$client" "$@"
fi
exec "$IAM_DB_OPS_SUDO_BIN" -n "$IAM_DB_OPS_DOCKER_BIN" run --rm --network host \
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
    backup|restore|status|retire-identity-dry-run|retire-identity-apply|reconcile-authn-dry-run|reconcile-authn-verify|reconcile-authn-apply|performance-schema-status) ;;
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
  if ! [[ "$MAX_BACKUP_AGE_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
    fail "backup age limit is invalid"
    return 1
  fi
  if [[ "$BACKUP_DIR" != /* ]] || [ -L "$BACKUP_DIR" ]; then
    fail "backup directory is invalid"
    return 1
  fi
  if ! [[ "$AUTHN_CONTAINER" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]]; then
    fail "AuthN maintenance container is invalid"
    return 1
  fi
  if ! [[ "$AUTHN_BATCH_SIZE" =~ ^[1-9][0-9]*$ ]] || [ "$AUTHN_BATCH_SIZE" -gt 50000 ]; then
    fail "AuthN reconciliation batch size is invalid"
    return 1
  fi
  if [ -n "$AUTHN_EVIDENCE_FILE" ]; then
    if [ "$OPERATION" != "reconcile-authn-verify" ] \
        || ! [[ "$AUTHN_EVIDENCE_FILE" =~ ^/tmp/iam-authn-retirement-[0-9]+-[0-9]+\.json$ ]] \
        || [ -L "$AUTHN_EVIDENCE_FILE" ]; then
      fail "AuthN reconciliation evidence path is invalid"
      return 1
    fi
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

validate_recent_backup() {
  validate_backup_name || return 1
  prepare_backup_directory || return 1

  local source_path="$BACKUP_DIR/$BACKUP_NAME"
  local modified_at now age
  if [ -L "$source_path" ] || [ ! -f "$source_path" ]; then
    fail "retirement backup file is unavailable"
    return 1
  fi
  if [ ! -s "$source_path" ] || ! gzip -t "$source_path" 2>/dev/null; then
    fail "retirement backup integrity validation failed"
    return 1
  fi
  modified_at="$(stat -c %Y "$source_path" 2>/dev/null || stat -f %m "$source_path" 2>/dev/null || true)"
  now="$(date +%s)"
  if ! [[ "$modified_at" =~ ^[0-9]+$ ]] || ! [[ "$now" =~ ^[0-9]+$ ]] || [ "$now" -lt "$modified_at" ]; then
    fail "retirement backup age is unavailable"
    return 1
  fi
  age=$((now - modified_at))
  if [ "$age" -gt "$MAX_BACKUP_AGE_SECONDS" ]; then
    fail "retirement backup is stale"
    return 1
  fi
  echo "identity retirement backup: result=valid age_seconds=$age max_age_seconds=$MAX_BACKUP_AGE_SECONDS"
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

  local database_size table_count backup_count latest_backup migration_state authn_table_state migration_lock_state other_authn_query_state other_authn_query_details
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
  if ! migration_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e 'SELECT COALESCE(MAX(version), -1), COALESCE(MAX(dirty + 0), -1), COUNT(*) FROM schema_migrations;' 2>"$ERROR_PATH")"; then
    fail "migration state query failed"
    return 1
  fi
  if ! authn_table_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "SELECT COALESCE(SUM(TABLE_NAME = 'auth_accounts'), 0), COALESCE(SUM(TABLE_NAME = 'auth_credentials_legacy'), 0) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('auth_accounts', 'auth_credentials_legacy');" 2>"$ERROR_PATH")"; then
    fail "AuthN table state query failed"
    return 1
  fi
  if ! migration_lock_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "WITH migration_lock AS (SELECT IS_USED_LOCK(CAST(MOD(CRC32(CONCAT(DATABASE(), ':schema_migrations')) * 1486364155, 4294967296) AS CHAR)) AS owner_id) SELECT COALESCE(CAST(migration_lock.owner_id AS CHAR), 'none'), CASE WHEN migration_lock.owner_id IS NULL THEN 'free' WHEN process.ID IS NULL THEN 'held_owner_not_visible' WHEN process.COMMAND = 'Sleep' THEN 'held_sleep' WHEN LOWER(COALESCE(process.INFO, '')) REGEXP '(^|[^a-z0-9_])(auth_accounts|auth_credentials_legacy)([^a-z0-9_]|$)' THEN 'held_legacy_authn_query' ELSE 'held_other_query' END, COALESCE(process.TIME, -1) FROM migration_lock LEFT JOIN information_schema.PROCESSLIST process ON process.ID = migration_lock.owner_id;" 2>"$ERROR_PATH")"; then
    fail "migration lock state query failed"
    return 1
  fi
  if ! other_authn_query_state="$(mysql_scalar "WITH migration_lock AS (
      SELECT IS_USED_LOCK(CAST(MOD(CRC32(CONCAT(DATABASE(), ':schema_migrations')) * 1486364155, 4294967296) AS CHAR)) AS owner_id
    )
    SELECT COUNT(*), COALESCE(MAX(process.TIME), -1)
    FROM information_schema.PROCESSLIST process
    CROSS JOIN migration_lock
    WHERE process.ID <> CONNECTION_ID()
      AND (migration_lock.owner_id IS NULL OR process.ID <> migration_lock.owner_id)
      AND process.COMMAND <> 'Sleep'
      AND LOWER(COALESCE(process.INFO, '')) REGEXP '(^|[^a-z0-9_])(auth_accounts|auth_credentials_legacy)([^a-z0-9_]|$)';")"; then
    fail "other AuthN query state query failed"
    return 1
  fi
  if ! other_authn_query_details="$(mysql_scalar "WITH migration_lock AS (
      SELECT IS_USED_LOCK(CAST(MOD(CRC32(CONCAT(DATABASE(), ':schema_migrations')) * 1486364155, 4294967296) AS CHAR)) AS owner_id
    )
    SELECT COALESCE(GROUP_CONCAT(CONCAT(process.ID, ':', process.TIME, ':',
      CASE
        WHEN TRIM(LOWER(COALESCE(process.INFO, ''))) REGEXP '^(select|with)' THEN 'read_only'
        ELSE 'non_read_only'
      END) ORDER BY process.TIME DESC SEPARATOR ','), 'none')
    FROM information_schema.PROCESSLIST process
    CROSS JOIN migration_lock
    WHERE process.ID <> CONNECTION_ID()
      AND (migration_lock.owner_id IS NULL OR process.ID <> migration_lock.owner_id)
      AND process.COMMAND <> 'Sleep'
      AND LOWER(COALESCE(process.INFO, '')) REGEXP '(^|[^a-z0-9_])(auth_accounts|auth_credentials_legacy)([^a-z0-9_]|$)';")"; then
    fail "other AuthN query detail query failed"
    return 1
  fi
  backup_count="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' | wc -l | tr -d ' ')"
  latest_backup="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' -print | sort -r | head -1 | sed -E 's/.*iam_backup_([0-9]{8}_[0-9]{6})\.sql\.gz/\1/' || true)"
  [ -n "$latest_backup" ] || latest_backup="none"
  echo "database status: result=success mysql_client=$MYSQL_CLIENT_VERSION connection=success size_mb=$database_size tables=$table_count backups=$backup_count latest_backup=$latest_backup"
  echo "migration status: schema_migrations=$migration_state authn_legacy_tables=$authn_table_state"
  echo "migration lock: owner_state=$migration_lock_state"
  echo "migration peers: other_legacy_authn_queries=$other_authn_query_state"
  echo "migration peer details: id_seconds_kind=$other_authn_query_details"
}

mysql_scalar() {
  local sql="$1"
  "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw \
    "$MYSQL_DBNAME" -e "$sql" 2>"$ERROR_PATH"
}

identity_retirement_dependency_count() {
  mysql_scalar "SELECT
    (SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE
      WHERE (TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships') AND REFERENCED_TABLE_NAME IS NOT NULL)
         OR (REFERENCED_TABLE_SCHEMA = DATABASE() AND REFERENCED_TABLE_NAME IN ('children', 'guardianships')))
    + (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (EVENT_OBJECT_TABLE IN ('children', 'guardianships')
          OR LOWER(ACTION_STATEMENT) REGEXP '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)'))
    + (SELECT COUNT(*) FROM information_schema.VIEWS
      WHERE TABLE_SCHEMA = DATABASE()
        AND (VIEW_DEFINITION IS NULL
          OR LOWER(VIEW_DEFINITION) REGEXP '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)'))
    + (SELECT COUNT(*) FROM information_schema.ROUTINES
      WHERE ROUTINE_SCHEMA = DATABASE()
        AND (ROUTINE_DEFINITION IS NULL
          OR LOWER(ROUTINE_DEFINITION) REGEXP '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)'))
    + (SELECT COUNT(*) FROM information_schema.EVENTS
      WHERE EVENT_SCHEMA = DATABASE()
        AND (EVENT_DEFINITION IS NULL
          OR LOWER(EVENT_DEFINITION) REGEXP '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)'));"
}

identity_retirement_gate() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  command -v gzip >/dev/null 2>&1 || { fail "gzip is unavailable"; return 1; }
  validate_recent_backup || return 1
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_identity_retirement.error"

  local migration_state table_state row_counts dependency_count
  if ! migration_state="$(mysql_scalar "SELECT COALESCE(MAX(version), -1), COALESCE(MAX(dirty + 0), -1), COUNT(*) FROM schema_migrations;")"; then
    fail "identity retirement migration state is unavailable"
    return 1
  fi
  if [ "$migration_state" != $'18\t0\t1' ]; then
    fail "identity retirement requires schema_migrations version 18 clean"
    return 1
  fi

  if ! table_state="$(mysql_scalar "SELECT
      COALESCE(SUM(TABLE_NAME = 'children' AND TABLE_TYPE = 'BASE TABLE'), 0),
      COALESCE(SUM(TABLE_NAME = 'guardianships' AND TABLE_TYPE = 'BASE TABLE'), 0)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships');")"; then
    fail "identity retirement table state is unavailable"
    return 1
  fi
  if [ "$table_state" = $'0\t0' ]; then
    echo "identity retirement gate: state=already_absent migration_version=18 dirty=0 reconciliation=waived_by_owner"
    return 0
  fi
  if [ "$table_state" != $'1\t1' ]; then
    fail "identity retirement found a partial legacy table state"
    return 1
  fi

  if ! dependency_count="$(identity_retirement_dependency_count)" || ! [[ "$dependency_count" =~ ^[0-9]+$ ]]; then
    fail "identity retirement dependency evidence is unavailable"
    return 1
  fi
  if [ "$dependency_count" != "0" ]; then
    fail "identity retirement database dependencies still exist"
    return 1
  fi
  if ! row_counts="$(mysql_scalar "SELECT (SELECT COUNT(*) FROM children), (SELECT COUNT(*) FROM guardianships);")"; then
    fail "identity retirement row counts are unavailable"
    return 1
  fi
  if ! [[ "$row_counts" =~ ^[0-9]+$'\t'[0-9]+$ ]]; then
    fail "identity retirement row counts are invalid"
    return 1
  fi

  local children_rows guardianship_rows
  IFS=$'\t' read -r children_rows guardianship_rows <<<"$row_counts"
  echo "identity retirement gate: state=eligible migration_version=18 dirty=0 dependencies=0 children_rows=$children_rows guardianships_rows=$guardianship_rows action=direct_drop reconciliation=waived_by_owner"
}

retire_identity_tables() {
  identity_retirement_gate || return 1
  if [ "$OPERATION" = "retire-identity-dry-run" ]; then
    echo "identity retirement completed: mode=dry-run result=success"
    return 0
  fi
  if [ "$CONFIRMATION" != "RETIRE_CHILDREN_GUARDIANSHIPS" ]; then
    fail "identity retirement confirmation is invalid"
    return 1
  fi

  local table_state result
  if ! table_state="$(mysql_scalar "SELECT
      COALESCE(SUM(TABLE_NAME = 'children' AND TABLE_TYPE = 'BASE TABLE'), 0),
      COALESCE(SUM(TABLE_NAME = 'guardianships' AND TABLE_TYPE = 'BASE TABLE'), 0)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships');")"; then
    fail "identity retirement table state is unavailable"
    return 1
  fi
  if [ "$table_state" = $'0\t0' ]; then
    echo "identity retirement completed: mode=apply result=already_absent"
    return 0
  fi
  if [ "$table_state" != $'1\t1' ]; then
    fail "identity retirement found a partial legacy table state"
    return 1
  fi

  SQL_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-identity-retirement.XXXXXX.sql")"
  chmod 0600 "$SQL_PATH"
  cat >"$SQL_PATH" <<'SQL'
SELECT GET_LOCK('iam_retire_children_guardianships', 0) INTO @iam_retirement_lock;

DROP TEMPORARY TABLE IF EXISTS iam_identity_retirement_assertion;
CREATE TEMPORARY TABLE iam_identity_retirement_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_identity_retirement_assertion (message)
VALUES ('identity retirement gate failed');
INSERT INTO iam_identity_retirement_assertion (message)
SELECT 'identity retirement gate failed'
WHERE @iam_retirement_lock <> 1;

LOCK TABLES children WRITE, guardianships WRITE, schema_migrations READ;

SET @iam_migration_invalid = (
    SELECT NOT (COUNT(*) = 1 AND MAX(version) = 18 AND MAX(dirty + 0) = 0)
    FROM schema_migrations
);
INSERT INTO iam_identity_retirement_assertion (message)
SELECT 'identity retirement gate failed'
WHERE @iam_migration_invalid <> 0;

SET @iam_retirement_pattern = '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)';
SET @iam_retirement_dependencies = (
      SELECT COUNT(*)
      FROM information_schema.KEY_COLUMN_USAGE
      WHERE (TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships') AND REFERENCED_TABLE_NAME IS NOT NULL)
         OR (REFERENCED_TABLE_SCHEMA = DATABASE() AND REFERENCED_TABLE_NAME IN ('children', 'guardianships'))
)
+ (
      SELECT COUNT(*)
      FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (EVENT_OBJECT_TABLE IN ('children', 'guardianships') OR LOWER(ACTION_STATEMENT) REGEXP @iam_retirement_pattern)
)
+ (
      SELECT COUNT(*)
      FROM information_schema.VIEWS
      WHERE TABLE_SCHEMA = DATABASE()
        AND (VIEW_DEFINITION IS NULL OR LOWER(VIEW_DEFINITION) REGEXP @iam_retirement_pattern)
)
+ (
      SELECT COUNT(*)
      FROM information_schema.ROUTINES
      WHERE ROUTINE_SCHEMA = DATABASE()
        AND (ROUTINE_DEFINITION IS NULL OR LOWER(ROUTINE_DEFINITION) REGEXP @iam_retirement_pattern)
)
+ (
      SELECT COUNT(*)
      FROM information_schema.EVENTS
      WHERE EVENT_SCHEMA = DATABASE()
        AND (EVENT_DEFINITION IS NULL OR LOWER(EVENT_DEFINITION) REGEXP @iam_retirement_pattern)
);
INSERT INTO iam_identity_retirement_assertion (message)
SELECT 'identity retirement gate failed'
WHERE @iam_retirement_dependencies <> 0;

SET @iam_children_rows = (SELECT COUNT(*) FROM children);
SET @iam_guardianships_rows = (SELECT COUNT(*) FROM guardianships);
DROP TEMPORARY TABLE iam_identity_retirement_assertion;

-- The owner explicitly waived legacy-to-canonical reconciliation. Keep this
-- as the only destructive statement and do not write canonical tables.
DROP TABLE children, guardianships;

DO RELEASE_LOCK('iam_retire_children_guardianships');
SELECT @iam_children_rows, @iam_guardianships_rows;
SQL

  echo "identity retirement started: mode=apply action=direct_drop reconciliation=waived_by_owner"
  if ! result="$("$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw \
      "$MYSQL_DBNAME" <"$SQL_PATH" 2>"$ERROR_PATH")"; then
    fail "identity retirement SQL did not complete; raw database errors were withheld"
    return 1
  fi
  if ! [[ "$result" =~ ^[0-9]+$'\t'[0-9]+$ ]]; then
    fail "identity retirement completion evidence is invalid"
    return 1
  fi
  local children_rows guardianship_rows
  IFS=$'\t' read -r children_rows guardianship_rows <<<"$result"
  echo "identity retirement completed: mode=apply result=success children_rows_deleted=$children_rows guardianships_rows_deleted=$guardianship_rows canonical_writes=0"
}

reconcile_authn_legacy() {
  local docker_bin sudo_bin running result required
  if [ "$OPERATION" = "reconcile-authn-apply" ]; then
    if [ "$CONFIRMATION" != "BACKFILL_AUTHN_LEGACY_MISSING" ]; then
      fail "AuthN reconciliation confirmation is invalid"
      return 1
    fi
    command -v gzip >/dev/null 2>&1 || { fail "gzip is unavailable"; return 1; }
    validate_recent_backup || return 1
  elif [ -n "${CONFIRMATION//[[:space:]]/}" ]; then
    fail "AuthN reconciliation dry-run does not accept confirmation"
    return 1
  fi

  docker_bin="$(command -v "${IAM_DB_OPS_DOCKER_BIN:-docker}" 2>/dev/null || true)"
  sudo_bin="$(command -v "${IAM_DB_OPS_SUDO_BIN:-sudo}" 2>/dev/null || true)"
  if [ -z "$docker_bin" ] || [ -z "$sudo_bin" ]; then
    fail "AuthN maintenance container access is unavailable"
    return 1
  fi
  if ! running="$($sudo_bin -n "$docker_bin" inspect --format '{{.State.Running}}' "$AUTHN_CONTAINER" 2>/dev/null)" \
      || [ "$running" != "true" ]; then
    fail "AuthN maintenance container is not running"
    return 1
  fi

  ERROR_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-authn-reconciliation.XXXXXX.error")"
  chmod 0600 "$ERROR_PATH"
  echo "AuthN reconciliation started: mode=${OPERATION#reconcile-authn-} canonical_policy=insert_missing_only batch_size=$AUTHN_BATCH_SIZE"
  if [ "$OPERATION" = "reconcile-authn-apply" ]; then
    if ! result="$($sudo_bin -n "$docker_bin" exec "$AUTHN_CONTAINER" \
        /app/iam-maintenance reconcile-authn-legacy --apply \
        --confirm=BACKFILL_AUTHN_LEGACY_MISSING --batch-size="$AUTHN_BATCH_SIZE" \
        --timeout=15m 2>"$ERROR_PATH")"; then
      [ -z "$result" ] || printf '%s\n' "$result"
      fail "AuthN reconciliation did not complete; raw runtime errors were withheld"
      return 1
    fi
  elif [ "$OPERATION" = "reconcile-authn-verify" ]; then
    if ! result="$($sudo_bin -n "$docker_bin" exec "$AUTHN_CONTAINER" \
        /app/iam-maintenance reconcile-authn-legacy --require-eligible \
        --timeout=15m 2>"$ERROR_PATH")"; then
      [ -z "$result" ] || printf '%s\n' "$result"
      fail "AuthN reconciliation did not complete; raw runtime errors were withheld"
      return 1
    fi
  else
    if ! result="$($sudo_bin -n "$docker_bin" exec "$AUTHN_CONTAINER" \
        /app/iam-maintenance reconcile-authn-legacy --timeout=15m 2>"$ERROR_PATH")"; then
      [ -z "$result" ] || printf '%s\n' "$result"
      fail "AuthN reconciliation did not complete; raw runtime errors were withheld"
      return 1
    fi
  fi
  if ! grep -Eq '"format_version"[[:space:]]*:[[:space:]]*5' <<<"$result" \
      || ! grep -Eq '"retirement_eligible"[[:space:]]*:[[:space:]]*(true|false)' <<<"$result"; then
    fail "AuthN reconciliation evidence is invalid"
    return 1
  fi
  if [ "$OPERATION" = "reconcile-authn-verify" ]; then
    for required in \
      '"hard_conflicts"[[:space:]]*:[[:space:]]*0' \
      '"remaining_login_identity_inserts"[[:space:]]*:[[:space:]]*0' \
      '"remaining_password_inserts"[[:space:]]*:[[:space:]]*0' \
      '"verification_required"[[:space:]]*:[[:space:]]*false' \
      '"retirement_eligible"[[:space:]]*:[[:space:]]*true'; do
      if ! grep -Eq "$required" <<<"$result"; then
        fail "AuthN reconciliation eligibility evidence is incomplete"
        return 1
      fi
    done
    if [ -n "$AUTHN_EVIDENCE_FILE" ]; then
      PARTIAL_PATH="$(mktemp "${AUTHN_EVIDENCE_FILE}.partial.XXXXXX")"
      chmod 0600 "$PARTIAL_PATH"
      printf '%s\n' "$result" >"$PARTIAL_PATH"
      mv -f -- "$PARTIAL_PATH" "$AUTHN_EVIDENCE_FILE"
      PARTIAL_PATH=""
    fi
  fi
  printf '%s\n' "$result"
  echo "AuthN reconciliation completed: mode=${OPERATION#reconcile-authn-} result=success"
}

performance_schema_status() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  prepare_backup_directory || return 1
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_performance_schema_status.error"

  local state grants privilege_state endpoint_provider endpoint_lower
  if ! state="$(mysql_scalar "SELECT
      @@performance_schema + 0,
      @@persisted_globals_load + 0,
      IF(TRIM(@@persist_only_admin_x509_subject) = '', 0, 1),
      COALESCE((SELECT MAX(VARIABLE_VALUE <> '') FROM performance_schema.session_status WHERE VARIABLE_NAME = 'Ssl_cipher'), 0),
      CASE
        WHEN LOWER(@@version_comment) REGEXP 'alibaba|rds|aurora|cloud sql|heatwave' THEN 'managed_or_cloud'
        WHEN LOWER(@@version_comment) LIKE '%community%' THEN 'community_or_self_managed'
        ELSE 'mysql_compatible_unknown'
      END,
      SUBSTRING_INDEX(@@version, '-', 1);")"; then
    fail "Performance Schema capability state is unavailable"
    return 1
  fi
  if ! [[ "$state" =~ ^[01]$'\t'[01]$'\t'[01]$'\t'[01]$'\t'[a-z_]+$'\t'[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "Performance Schema capability state is invalid"
    return 1
  fi

  privilege_state="not_visible"
  if grants="$(mysql_scalar "SHOW GRANTS FOR CURRENT_USER;")"; then
    if grep -Eqi 'ALL PRIVILEGES ON \*\.\*|SYSTEM_VARIABLES_ADMIN' <<<"$grants" \
      && grep -Eqi 'ALL PRIVILEGES ON \*\.\*|PERSIST_RO_VARIABLES_ADMIN' <<<"$grants"; then
      privilege_state="visible"
    fi
  fi

  endpoint_lower="$(tr '[:upper:]' '[:lower:]' <<<"${MYSQL_HOST:-}")"
  case "$endpoint_lower" in
    *.mysql.rds.aliyuncs.com|*.rds.aliyuncs.com) endpoint_provider="aliyun_rds" ;;
    *.rds.amazonaws.com) endpoint_provider="aws_rds" ;;
    *.cloudsql.googleusercontent.com) endpoint_provider="google_cloud_sql" ;;
    127.0.0.1|localhost|::1) endpoint_provider="local" ;;
    *) endpoint_provider="unknown" ;;
  esac

  local enabled persisted_load x509_configured tls_active server_flavor server_version
  IFS=$'\t' read -r enabled persisted_load x509_configured tls_active server_flavor server_version <<<"$state"
  echo "performance schema capability: result=success enabled=$enabled persisted_globals_load=$persisted_load persist_x509_subject_configured=$x509_configured tls_active=$tls_active persist_privileges=$privilege_state server_flavor=$server_flavor endpoint_provider=$endpoint_provider server_version=$server_version"

  local table_io_contract table_io_probe table_io_select table_io_metadata_visible table_io_required_columns
  table_io_contract="unavailable"
  table_io_select="not_checked"
  table_io_metadata_visible="unknown"
  table_io_required_columns="unknown"
  if table_io_probe="$(mysql_scalar "SELECT
      EXISTS(SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = 'performance_schema'
          AND TABLE_NAME = 'table_io_waits_summary_by_table'),
      (SELECT COUNT(*) FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = 'performance_schema'
          AND TABLE_NAME = 'table_io_waits_summary_by_table'
          AND COLUMN_NAME IN ('COUNT_STAR', 'SUM_TIMER_WAIT', 'COUNT_READ', 'COUNT_WRITE'));" )" \
      && [[ "$table_io_probe" =~ ^[01]$'\t'[0-9]+$ ]]; then
    IFS=$'\t' read -r table_io_metadata_visible table_io_required_columns <<<"$table_io_probe"
    if [ "$table_io_metadata_visible" = "1" ] && [ "$table_io_required_columns" = "4" ]; then
      table_io_contract="valid"
    else
      table_io_contract="invalid"
    fi
  fi
  if table_io_probe="$(mysql_scalar "SELECT COUNT(*) FROM performance_schema.table_io_waits_summary_by_table;")" \
      && [[ "$table_io_probe" =~ ^[0-9]+$ ]]; then
    table_io_select="available"
  else
    table_io_select="unavailable"
  fi
  echo "performance schema capability: table_io_contract=$table_io_contract table_io_metadata_visible=$table_io_metadata_visible table_io_required_columns=$table_io_required_columns table_io_select=$table_io_select"

  local sys_io_probe sys_io_select table_statistics_probe table_statistics_enabled table_statistics_select
  sys_io_select="unavailable"
  if sys_io_probe="$(mysql_scalar "SELECT COUNT(*) FROM sys.schema_table_statistics;")" \
      && [[ "$sys_io_probe" =~ ^[0-9]+$ ]]; then
    sys_io_select="available"
  fi
  echo "performance schema capability: sys_table_statistics_select=$sys_io_select"

  table_statistics_enabled="unavailable"
  table_statistics_select="unavailable"
  if table_statistics_probe="$(mysql_scalar "SELECT @@opt_tablestat + 0;")" \
      && [[ "$table_statistics_probe" =~ ^[01]$ ]]; then
    table_statistics_enabled="$table_statistics_probe"
  fi
  if table_statistics_probe="$(mysql_scalar "SELECT COUNT(*) FROM information_schema.TABLE_STATISTICS;")" \
      && [[ "$table_statistics_probe" =~ ^[0-9]+$ ]]; then
    table_statistics_select="available"
  fi
  echo "performance schema capability: rds_table_statistics_enabled=$table_statistics_enabled rds_table_statistics_select=$table_statistics_select"

  if [ "$enabled" = "1" ]; then
    echo "performance schema capability: next_action=already_enabled restart_required=0"
  elif [ "$persisted_load" = "1" ] && [ "$x509_configured" = "1" ] && [ "$tls_active" = "1" ] && [ "$privilege_state" = "visible" ]; then
    echo "performance schema capability: next_action=persist_only_then_controlled_restart restart_required=1"
  else
    echo "performance schema capability: next_action=configure_provider_or_server_startup restart_required=1"
  fi
}

main() {
  umask 077
  validate_configuration || return 1
  prepare_container_client_fallback || return 1
  case "$OPERATION" in
    backup) backup_database ;;
    restore) restore_database ;;
    status) database_status ;;
    retire-identity-dry-run|retire-identity-apply) retire_identity_tables ;;
    reconcile-authn-dry-run|reconcile-authn-verify|reconcile-authn-apply) reconcile_authn_legacy ;;
    performance-schema-status) performance_schema_status ;;
  esac
}

main
