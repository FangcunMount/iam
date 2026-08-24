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
REPORT_PARTIAL_PATH=""
ROLEBINDING_FINGERPRINT="${IAM_DB_OPS_ROLEBINDING_FINGERPRINT:-}"
ROLEBINDING_CANDIDATE_COUNT="${IAM_DB_OPS_ROLEBINDING_CANDIDATE_COUNT:-}"

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
  if [ -n "$REPORT_PARTIAL_PATH" ]; then
    rm -f -- "$REPORT_PARTIAL_PATH"
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
    backup|restore|status|performance-schema-status|rolebinding-guard-preflight|rolebinding-deduplicate-dry-run|rolebinding-deduplicate-apply) ;;
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
  if [ "$OPERATION" = "rolebinding-deduplicate-apply" ]; then
    validate_backup_name || return 1
    if ! [[ "$ROLEBINDING_FINGERPRINT" =~ ^[0-9a-f]{64}$ ]]; then
      fail "RoleBinding candidate fingerprint is invalid"
      return 1
    fi
    if ! [[ "$ROLEBINDING_CANDIDATE_COUNT" =~ ^[1-9][0-9]*$ ]]; then
      fail "RoleBinding candidate count is invalid"
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

  local database_size table_count schema_objects schema_guard_state backup_count latest_backup migration_state retired_table_state retired_table_privilege_state migration_lock_state
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
  if ! schema_objects="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "SELECT CONCAT('type=', REPLACE(TABLE_TYPE, ' ', '_'), ' name=', TABLE_NAME) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_TYPE, TABLE_NAME;" 2>"$ERROR_PATH")"; then
    fail "database schema inventory query failed"
    return 1
  fi
  if ! schema_guard_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "/* iam_schema_guard */ SELECT COALESCE(SUM(TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME IN ('auth_credentials', 'auth_login_identities', 'authz_assignments', 'authz_policy_versions', 'authz_resources', 'authz_roles', 'casbin_rule', 'domain_event_outbox', 'identity_session_revocation_outbox', 'idp_wechat_apps', 'jwks_keys', 'profile_links', 'profiles', 'schema_migrations', 'users')), 0), COUNT(*), COALESCE(SUM(NOT (TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME IN ('auth_credentials', 'auth_login_identities', 'authz_assignments', 'authz_policy_versions', 'authz_resources', 'authz_roles', 'casbin_rule', 'domain_event_outbox', 'identity_session_revocation_outbox', 'idp_wechat_apps', 'jwks_keys', 'profile_links', 'profiles', 'schema_migrations', 'users'))), 0) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE();" 2>"$ERROR_PATH")"; then
    fail "database schema guard query failed"
    return 1
  fi
  if ! migration_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e 'SELECT COALESCE(MAX(version), -1), COALESCE(MAX(dirty + 0), -1), COUNT(*) FROM schema_migrations;' 2>"$ERROR_PATH")"; then
    fail "migration state query failed"
    return 1
  fi
  if ! retired_table_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "/* iam_retired_table_guard */ SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships', 'schema_version', 'tenants', 'data_dictionary', 'operation_logs', 'audit_logs', 'auth_token_audit', 'auth_accounts', 'auth_credentials_legacy', 'cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1');" 2>"$ERROR_PATH")"; then
    fail "retired table state query failed"
    return 1
  fi
  if ! retired_table_privilege_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "/* iam_retired_privilege_guard */ SELECT COUNT(*) FROM information_schema.TABLE_PRIVILEGES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('children', 'guardianships', 'schema_version', 'tenants', 'data_dictionary', 'operation_logs', 'audit_logs', 'auth_token_audit', 'auth_accounts', 'auth_credentials_legacy', 'cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1');" 2>"$ERROR_PATH")"; then
    fail "retired table privilege state query failed"
    return 1
  fi
  if ! migration_lock_state="$($MYSQL_BIN --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names "$MYSQL_DBNAME" -e "WITH migration_lock AS (SELECT IS_USED_LOCK(CAST(MOD(CRC32(CONCAT(DATABASE(), ':schema_migrations')) * 1486364155, 4294967296) AS CHAR)) AS owner_id) SELECT COALESCE(CAST(migration_lock.owner_id AS CHAR), 'none'), CASE WHEN migration_lock.owner_id IS NULL THEN 'free' WHEN process.ID IS NULL THEN 'held_owner_not_visible' WHEN process.COMMAND = 'Sleep' THEN 'held_sleep' ELSE 'held_query' END, COALESCE(process.TIME, -1) FROM migration_lock LEFT JOIN information_schema.PROCESSLIST process ON process.ID = migration_lock.owner_id;" 2>"$ERROR_PATH")"; then
    fail "migration lock state query failed"
    return 1
  fi
  backup_count="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' | wc -l | tr -d ' ')"
  latest_backup="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iam_backup_????????_??????.sql.gz' -print | sort -r | head -1 | sed -E 's/.*iam_backup_([0-9]{8}_[0-9]{6})\.sql\.gz/\1/' || true)"
  [ -n "$latest_backup" ] || latest_backup="none"
  echo "database status: result=success mysql_client=$MYSQL_CLIENT_VERSION connection=success size_mb=$database_size tables=$table_count backups=$backup_count latest_backup=$latest_backup"
  printf 'schema objects:\n%s\n' "$schema_objects"
  echo "migration status: schema_migrations=$migration_state retired_tables_present=$retired_table_state retired_table_privileges=$retired_table_privilege_state"
  echo "migration lock: owner_state=$migration_lock_state"
  if [ "$migration_state" != $'25\t0\t1' ]; then
    fail "migration status is not version 25 clean"
    return 1
  fi
  if [ "$schema_guard_state" != $'15\t15\t0' ]; then
    fail "database schema differs from the 15-table allowlist"
    return 1
  fi
  if [ "$retired_table_state" != "0" ]; then
    fail "retired tables are present"
    return 1
  fi
  if [ "$retired_table_privilege_state" != "0" ]; then
    fail "retired table privileges are present"
    return 1
  fi
  echo "schema guard: result=success required_base_tables=15 schema_objects=15 unexpected_objects=0"
  echo "retirement guard: result=success expected_version=25 retired_tables_present=0 retired_table_privileges=0"
}

mysql_scalar() {
  local sql="$1"
  "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw \
    "$MYSQL_DBNAME" -e "$sql" 2>"$ERROR_PATH"
}

rolebinding_guard_preflight() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  prepare_backup_directory || return 1
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_rolebinding_guard_preflight.error"

  local migration_state duplicate_state guard_state version dirty row_count
  if ! migration_state="$(mysql_scalar 'SELECT COALESCE(MAX(version), -1), COALESCE(MAX(dirty + 0), -1), COUNT(*) FROM schema_migrations;')"; then
    fail "migration state query failed"
    return 1
  fi
  if ! [[ "$migration_state" =~ ^[0-9]+$'\t'[01]$'\t'[0-9]+$ ]]; then
    fail "migration state is invalid"
    return 1
  fi
  IFS=$'\t' read -r version dirty row_count <<<"$migration_state"
  if { [ "$version" != "24" ] && [ "$version" != "25" ]; } || [ "$dirty" != "0" ] || [ "$row_count" != "1" ]; then
    fail "RoleBinding guard preflight requires clean migration version 24 or 25"
    return 1
  fi

  if ! duplicate_state="$(mysql_scalar "SELECT COUNT(*), COALESCE(SUM(duplicate_count - 1), 0), COALESCE(MAX(duplicate_count), 0) FROM (SELECT COUNT(*) AS duplicate_count FROM authz_assignments WHERE deleted_at IS NULL GROUP BY subject_type, subject_id, role_id, tenant_id HAVING COUNT(*) > 1) AS duplicate_groups;")"; then
    fail "active RoleBinding duplicate query failed"
    return 1
  fi
  if [ "$duplicate_state" != $'0\t0\t0' ]; then
    echo "rolebinding guard preflight: result=blocked migration_version=$version duplicate_state=$duplicate_state"
    fail "duplicate active RoleBindings must be resolved before migration 000025"
    return 1
  fi

  if ! guard_state="$(mysql_scalar "SELECT (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authz_assignments' AND COLUMN_NAME = 'active_guard'), (SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authz_assignments' AND INDEX_NAME = 'uk_authz_assignments_active');")"; then
    fail "RoleBinding guard schema query failed"
    return 1
  fi
  case "$version:$guard_state" in
    24:$'0\t0'|25:$'1\t1') ;;
    *)
      fail "RoleBinding guard schema is inconsistent with migration version"
      return 1
      ;;
  esac

  echo "rolebinding guard preflight: result=success migration_version=$version duplicate_groups=0 duplicate_extra_rows=0 max_group_size=0 guard_state=$guard_state"
}

rolebinding_candidate_query() {
  cat <<'SQL'
WITH ranked AS (
  SELECT
    id,
    subject_type,
    subject_id,
    role_id,
    tenant_id,
    granted_at,
    created_at,
    FIRST_VALUE(id) OVER duplicate_window AS keep_id,
    ROW_NUMBER() OVER duplicate_window AS duplicate_rank
  FROM authz_assignments
  WHERE deleted_at IS NULL
  WINDOW duplicate_window AS (
    PARTITION BY subject_type, subject_id, role_id, tenant_id
    ORDER BY granted_at ASC, created_at ASC, id ASC
  )
)
SELECT
  id,
  keep_id,
  HEX(subject_type),
  HEX(subject_id),
  role_id,
  HEX(tenant_id),
  DATE_FORMAT(granted_at, '%Y-%m-%dT%H:%i:%s'),
  DATE_FORMAT(created_at, '%Y-%m-%dT%H:%i:%s')
FROM ranked
WHERE duplicate_rank > 1
ORDER BY id ASC;
SQL
}

require_rolebinding_cleanup_schema() {
  local migration_state guard_state
  if ! migration_state="$(mysql_scalar 'SELECT COALESCE(MAX(version), -1), COALESCE(MAX(dirty + 0), -1), COUNT(*) FROM schema_migrations;')"; then
    fail "migration state query failed"
    return 1
  fi
  if [ "$migration_state" != $'24\t0\t1' ]; then
    fail "RoleBinding deduplication requires clean migration version 24"
    return 1
  fi
  if ! guard_state="$(mysql_scalar "SELECT (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authz_assignments' AND COLUMN_NAME = 'active_guard'), (SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authz_assignments' AND INDEX_NAME = 'uk_authz_assignments_active');")"; then
    fail "RoleBinding guard schema query failed"
    return 1
  fi
  if [ "$guard_state" != $'0\t0' ]; then
    fail "RoleBinding deduplication refuses a partially or fully applied guard"
    return 1
  fi
}

rolebinding_deduplicate_dry_run() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  command -v sha256sum >/dev/null 2>&1 || { fail "sha256sum is unavailable"; return 1; }
  prepare_backup_directory || return 1
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_rolebinding_deduplicate_dry_run.error"
  require_rolebinding_cleanup_schema || return 1

  local timestamp report_path candidate_count candidate_ids fingerprint duplicate_state
  timestamp="$(new_backup_timestamp)"
  if ! [[ "$timestamp" =~ ^[0-9]{8}_[0-9]{6}$ ]]; then
    fail "RoleBinding report timestamp is invalid"
    return 1
  fi
  report_path="$BACKUP_DIR/rolebinding_deduplicate_${timestamp}.tsv"
  if [ -e "$report_path" ]; then
    fail "RoleBinding report destination already exists"
    return 1
  fi
  REPORT_PARTIAL_PATH="$BACKUP_DIR/.rolebinding_deduplicate_${timestamp}.tsv.partial"
  if ! rolebinding_candidate_query | "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" \
      --batch --skip-column-names --raw "$MYSQL_DBNAME" >"$REPORT_PARTIAL_PATH" 2>"$ERROR_PATH"; then
    fail "RoleBinding candidate report query failed"
    return 1
  fi
  chmod 0600 "$REPORT_PARTIAL_PATH"
  if [ -s "$REPORT_PARTIAL_PATH" ] && ! awk -F '\t' 'NF != 8 || $1 !~ /^[0-9]+$/ || $2 !~ /^[0-9]+$/ { exit 1 }' "$REPORT_PARTIAL_PATH"; then
    fail "RoleBinding candidate report is invalid"
    return 1
  fi
  candidate_count="$(wc -l <"$REPORT_PARTIAL_PATH" | tr -d ' ')"
  candidate_ids="$(cut -f1 "$REPORT_PARTIAL_PATH" | paste -sd, -)"
  fingerprint="$(printf '%s' "$candidate_ids" | sha256sum | awk '{print $1}')"
  if ! [[ "$fingerprint" =~ ^[0-9a-f]{64}$ ]]; then
    fail "RoleBinding candidate fingerprint could not be calculated"
    return 1
  fi
  if ! duplicate_state="$(mysql_scalar "SELECT COUNT(*), COALESCE(SUM(duplicate_count - 1), 0), COALESCE(MAX(duplicate_count), 0) FROM (SELECT COUNT(*) AS duplicate_count FROM authz_assignments WHERE deleted_at IS NULL GROUP BY subject_type, subject_id, role_id, tenant_id HAVING COUNT(*) > 1) AS duplicate_groups;")"; then
    fail "active RoleBinding duplicate query failed"
    return 1
  fi
  if [ "$candidate_count" = "0" ]; then
    fail "RoleBinding deduplication has no active duplicate candidates"
    return 1
  fi
  if [ "$duplicate_state" != "$(printf '%s' "$duplicate_state" | awk -F '\t' -v count="$candidate_count" '$2 == count { print $0 }')" ]; then
    fail "RoleBinding candidate count is inconsistent with duplicate state"
    return 1
  fi
  mv -- "$REPORT_PARTIAL_PATH" "$report_path"
  REPORT_PARTIAL_PATH=""

  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'rolebinding_deduplicate_????????_??????.tsv' -print |
    sort -r |
    awk 'NR > 10' |
    while IFS= read -r old; do
      [ -n "$old" ] && rm -f -- "$old"
    done

  echo "rolebinding deduplicate dry-run: result=success candidate_count=$candidate_count fingerprint=$fingerprint duplicate_state=$duplicate_state report=$(basename "$report_path") keep_order=granted_at,created_at,id"
}

require_recent_backup() {
  prepare_backup_directory || return 1
  local source_path="$BACKUP_DIR/$BACKUP_NAME" now modified age
  if [ -L "$source_path" ] || [ ! -f "$source_path" ]; then
    fail "required pre-operation backup is unavailable"
    return 1
  fi
  if ! gzip -t "$source_path" 2>/dev/null; then
    fail "required pre-operation backup failed integrity validation"
    return 1
  fi
  now="$(date +%s)"
  modified="$(stat -c %Y "$source_path" 2>/dev/null || true)"
  if ! [[ "$modified" =~ ^[0-9]+$ ]]; then
    fail "required pre-operation backup age is unavailable"
    return 1
  fi
  age=$((now - modified))
  if [ "$age" -lt 0 ] || [ "$age" -gt 86400 ]; then
    fail "required pre-operation backup is older than 24 hours"
    return 1
  fi
}

rolebinding_deduplicate_apply() {
  require_mysql8_client "$MYSQL_BIN" mysql || return 1
  command -v gzip >/dev/null 2>&1 || { fail "gzip is unavailable"; return 1; }
  prepare_defaults_file
  ERROR_PATH="$BACKUP_DIR/.iam_rolebinding_deduplicate_apply.error"
  require_rolebinding_cleanup_schema || return 1
  require_recent_backup || return 1

  local apply_state
  if ! apply_state="$("$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw "$MYSQL_DBNAME" 2>"$ERROR_PATH" <<SQL
SET SESSION group_concat_max_len = 1048576;
SET autocommit = 0;
LOCK TABLES authz_assignments AS binding WRITE;
CREATE TEMPORARY TABLE rolebinding_deduplicate_candidates (
  id BIGINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE=InnoDB;
CREATE TEMPORARY TABLE rolebinding_deduplicate_guard (
  value TINYINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE=InnoDB;
INSERT INTO rolebinding_deduplicate_guard (value) VALUES (1);
INSERT INTO rolebinding_deduplicate_candidates (id)
SELECT id
FROM (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY subject_type, subject_id, role_id, tenant_id
      ORDER BY granted_at ASC, created_at ASC, id ASC
    ) AS duplicate_rank
  FROM authz_assignments AS binding
  WHERE binding.deleted_at IS NULL
) AS ranked
WHERE duplicate_rank > 1;
SET @candidate_count = (SELECT COUNT(*) FROM rolebinding_deduplicate_candidates);
SET @candidate_hash = (
  SELECT COALESCE(
    SHA2(GROUP_CONCAT(CAST(id AS CHAR) ORDER BY id ASC SEPARATOR ','), 256),
    SHA2('', 256)
  )
  FROM rolebinding_deduplicate_candidates
);
INSERT INTO rolebinding_deduplicate_guard (value)
SELECT 1
WHERE NOT (
  @candidate_count = ${ROLEBINDING_CANDIDATE_COUNT}
  AND @candidate_hash = '${ROLEBINDING_FINGERPRINT}'
);
UPDATE authz_assignments AS binding
INNER JOIN rolebinding_deduplicate_candidates AS candidate ON candidate.id = binding.id
SET
  binding.deleted_at = CURRENT_TIMESTAMP,
  binding.updated_at = CURRENT_TIMESTAMP,
  binding.deleted_by = 0,
  binding.updated_by = 0,
  binding.version = binding.version + 1
WHERE binding.deleted_at IS NULL;
SET @affected_rows = ROW_COUNT();
INSERT INTO rolebinding_deduplicate_guard (value)
SELECT 1
WHERE NOT (@affected_rows = @candidate_count);
SET @duplicate_groups = (
  SELECT COUNT(*)
  FROM (
    SELECT 1
    FROM authz_assignments AS binding
    WHERE binding.deleted_at IS NULL
    GROUP BY binding.subject_type, binding.subject_id, binding.role_id, binding.tenant_id
    HAVING COUNT(*) > 1
  ) AS remaining_duplicate_groups
);
INSERT INTO rolebinding_deduplicate_guard (value)
SELECT 1
WHERE NOT (@duplicate_groups = 0);
COMMIT;
SELECT @candidate_count, @candidate_hash, @affected_rows, @duplicate_groups;
UNLOCK TABLES;
SQL
)"; then
    fail "RoleBinding deduplication apply failed"
    return 1
  fi
  if [ "$apply_state" != "${ROLEBINDING_CANDIDATE_COUNT}"$'\t'"${ROLEBINDING_FINGERPRINT}"$'\t'"${ROLEBINDING_CANDIDATE_COUNT}"$'\t0' ]; then
    fail "RoleBinding deduplication apply result is invalid"
    return 1
  fi
  echo "rolebinding deduplicate apply: result=success candidate_count=$ROLEBINDING_CANDIDATE_COUNT fingerprint=$ROLEBINDING_FINGERPRINT soft_deleted=$ROLEBINDING_CANDIDATE_COUNT duplicate_groups=0 backup=$BACKUP_NAME"
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
    performance-schema-status) performance_schema_status ;;
    rolebinding-guard-preflight) rolebinding_guard_preflight ;;
    rolebinding-deduplicate-dry-run) rolebinding_deduplicate_dry_run ;;
    rolebinding-deduplicate-apply) rolebinding_deduplicate_apply ;;
  esac
}

main
