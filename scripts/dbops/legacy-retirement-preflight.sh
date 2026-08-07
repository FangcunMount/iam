#!/usr/bin/env bash

set -euo pipefail

MYSQL_BIN="${IAM_RETIREMENT_MYSQL_BIN:-mysql}"
ENVIRONMENT="${IAM_RETIREMENT_ENVIRONMENT:-}"
COMMIT_SHA="${IAM_RETIREMENT_COMMIT_SHA:-}"
IMAGE_SHA="${IAM_RETIREMENT_IMAGE_SHA:-unknown}"
SUPPLIED_DEFAULTS="${IAM_RETIREMENT_MYSQL_DEFAULTS_FILE:-}"
ALLOW_DOCKER_CLIENT="${IAM_RETIREMENT_ALLOW_DOCKER_CLIENT:-0}"
MYSQL_CLIENT_IMAGE="${IAM_RETIREMENT_MYSQL_CLIENT_IMAGE:-mysql:8.0}"
RETIREMENT_SCOPE="${IAM_RETIREMENT_SCOPE:-all}"
OWNER_IO_WAIVER="${IAM_RETIREMENT_OWNER_IO_WAIVER:-none}"

MYSQL_DEFAULTS=""
ERROR_PATH=""
IO_CACHE_PATH=""
DEPENDENCY_CACHE_PATH=""
PARITY_CACHE_PATH=""
CLIENT_WRAPPER_DIR=""
OWNS_DEFAULTS=0

MIGRATION_PRESENT=0
MIGRATION_VERSION=unknown
MIGRATION_DIRTY=unknown
PERFORMANCE_ENABLED=0

CANDIDATE_TABLES="children guardianships auth_accounts auth_credentials_legacy schema_version tenants data_dictionary operation_logs audit_logs auth_token_audit"

fail() {
  printf 'legacy retirement preflight failed: %s\n' "$1" >&2
  return 1
}

cleanup() {
  if [ "$OWNS_DEFAULTS" -eq 1 ] && [ -n "$MYSQL_DEFAULTS" ]; then
    rm -f -- "$MYSQL_DEFAULTS"
  fi
  if [ -n "$ERROR_PATH" ]; then
    rm -f -- "$ERROR_PATH"
  fi
  if [ -n "$IO_CACHE_PATH" ]; then
    rm -f -- "$IO_CACHE_PATH"
  fi
  if [ -n "$DEPENDENCY_CACHE_PATH" ]; then
    rm -f -- "$DEPENDENCY_CACHE_PATH"
  fi
  if [ -n "$PARITY_CACHE_PATH" ]; then
    rm -f -- "$PARITY_CACHE_PATH"
  fi
  if [ -n "$CLIENT_WRAPPER_DIR" ]; then
    rm -f -- "$CLIENT_WRAPPER_DIR/mysql"
    rmdir -- "$CLIENT_WRAPPER_DIR" 2>/dev/null || true
  fi
}

prepare_container_client_fallback() {
  local docker_bin sudo_bin wrapper
  if command -v "$MYSQL_BIN" >/dev/null 2>&1; then
    return
  fi
  if [ "$ALLOW_DOCKER_CLIENT" != "1" ]; then
    return
  fi
  if ! [[ "$MYSQL_CLIENT_IMAGE" =~ ^[A-Za-z0-9._/@:-]+$ ]]; then
    fail "MySQL client image is invalid"
    return 1
  fi
  docker_bin="$(command -v "${IAM_RETIREMENT_DOCKER_BIN:-docker}" 2>/dev/null || true)"
  sudo_bin="$(command -v "${IAM_RETIREMENT_SUDO_BIN:-sudo}" 2>/dev/null || true)"
  if [ -z "$docker_bin" ] || [ -z "$sudo_bin" ] || ! "$sudo_bin" -n "$docker_bin" info >/dev/null 2>&1; then
    fail "containerized MySQL client is unavailable"
    return 1
  fi

  CLIENT_WRAPPER_DIR="$(mktemp -d "${TMPDIR:-/tmp}/iam-retirement-mysql-wrapper.XXXXXX")"
  chmod 0700 "$CLIENT_WRAPPER_DIR"
  export IAM_RETIREMENT_DOCKER_BIN="$docker_bin"
  export IAM_RETIREMENT_SUDO_BIN="$sudo_bin"
  export IAM_RETIREMENT_MYSQL_CLIENT_IMAGE="$MYSQL_CLIENT_IMAGE"
  wrapper="$CLIENT_WRAPPER_DIR/mysql"
  cat >"$wrapper" <<'EOF'
#!/bin/sh
set -eu
defaults=""
for argument in "$@"; do
  case "$argument" in
    --defaults-extra-file=*) defaults="${argument#*=}" ;;
  esac
done
if [ -n "$defaults" ]; then
  exec "$IAM_RETIREMENT_SUDO_BIN" -n "$IAM_RETIREMENT_DOCKER_BIN" run --rm --network host \
    --volume "$defaults:$defaults:ro" "$IAM_RETIREMENT_MYSQL_CLIENT_IMAGE" mysql "$@"
fi
exec "$IAM_RETIREMENT_SUDO_BIN" -n "$IAM_RETIREMENT_DOCKER_BIN" run --rm --network host \
  "$IAM_RETIREMENT_MYSQL_CLIENT_IMAGE" mysql "$@"
EOF
  chmod 0700 "$wrapper"
  MYSQL_BIN="$wrapper"
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

validate_label() {
  local name="$1"
  local value="$2"
  if ! [[ "$value" =~ ^[A-Za-z0-9._:@+-]+$ ]]; then
    fail "metadata label is invalid ($name)"
    return 1
  fi
}

validate_configuration() {
  require_value IAM_RETIREMENT_ENVIRONMENT "$ENVIRONMENT" || return 1
  require_value MYSQL_DBNAME "${MYSQL_DBNAME:-}" || return 1
  validate_label IAM_RETIREMENT_ENVIRONMENT "$ENVIRONMENT" || return 1
  validate_label IAM_RETIREMENT_IMAGE_SHA "$IMAGE_SHA" || return 1
  case "$RETIREMENT_SCOPE" in
    all|identity|schema_version|platform|authn) ;;
    *) fail "retirement scope is invalid"; return 1 ;;
  esac
  case "$OWNER_IO_WAIVER" in
    none|platform_tables) ;;
    *) fail "owner I/O waiver is invalid"; return 1 ;;
  esac
  if ! [[ "$MYSQL_DBNAME" =~ ^[A-Za-z0-9_]+$ ]]; then
    fail "database name is invalid"
    return 1
  fi

  if [ -n "$SUPPLIED_DEFAULTS" ]; then
    if [[ "$SUPPLIED_DEFAULTS" != /* ]] || [ ! -f "$SUPPLIED_DEFAULTS" ] || [ -L "$SUPPLIED_DEFAULTS" ]; then
      fail "MySQL defaults file is invalid"
      return 1
    fi
  else
    require_value MYSQL_HOST "${MYSQL_HOST:-}" || return 1
    require_value MYSQL_USERNAME "${MYSQL_USERNAME:-}" || return 1
    require_value MYSQL_PASSWORD "${MYSQL_PASSWORD:-}" || return 1
    if ! [[ "${MYSQL_PORT:-3306}" =~ ^[0-9]+$ ]]; then
      fail "database port is invalid"
      return 1
    fi
  fi

  if [ -n "$COMMIT_SHA" ]; then
    validate_label IAM_RETIREMENT_COMMIT_SHA "$COMMIT_SHA" || return 1
  elif command -v git >/dev/null 2>&1; then
    COMMIT_SHA="$(git rev-parse HEAD 2>/dev/null || true)"
  fi
  [ -n "$COMMIT_SHA" ] || COMMIT_SHA="unknown"
  validate_label IAM_RETIREMENT_COMMIT_SHA "$COMMIT_SHA" || return 1
}

emit_identity_schema_contracts() {
  local table expected expected_count result actual_count actual_columns
  while IFS='|' read -r table expected; do
    expected_count="$(awk -F ',' '{ print NF }' <<<"$expected")"
    result="$(mysql_query "SELECT
      COUNT(*),
      COALESCE(GROUP_CONCAT(COLUMN_NAME ORDER BY ORDINAL_POSITION SEPARATOR ','), 'none')
      FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = '$table';")"
    IFS=$'\t' read -r actual_count actual_columns <<<"$result"
    printf 'schema_contract\t%s\texpected_columns=%s\tactual_columns=%s\tcolumn_names=%s\n' \
      "$table" "$expected_count" "$actual_count" "$actual_columns"
  done <<'EOF'
children|id,name,id_card,gender,birthday,height,weight,created_at,updated_at,deleted_at,created_by,updated_by,deleted_by,version
profiles|id,name,id_card,gender,birthday,height,weight,created_at,updated_at,deleted_at,created_by,updated_by,deleted_by,version
guardianships|id,user_id,child_id,relation,established_at,revoked_at,created_at,updated_at,deleted_at,created_by,updated_by,deleted_by,version
profile_links|id,user_id,profile_id,type,relation,established_at,revoked_at,created_at,updated_at,deleted_at,created_by,updated_by,deleted_by,version
EOF
}

require_mysql8_client() {
  local version
  if ! command -v "$MYSQL_BIN" >/dev/null 2>&1; then
    fail "mysql client is unavailable"
    return 1
  fi
  if ! version="$($MYSQL_BIN --version 2>/dev/null)" || ! grep -Eq 'Ver 8\.' <<<"$version"; then
    fail "mysql client must be MySQL 8.x"
    return 1
  fi
}

prepare_defaults_file() {
  if [ -n "$SUPPLIED_DEFAULTS" ]; then
    MYSQL_DEFAULTS="$SUPPLIED_DEFAULTS"
    return
  fi
  MYSQL_DEFAULTS="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-mysql.XXXXXX")"
  OWNS_DEFAULTS=1
  chmod 0600 "$MYSQL_DEFAULTS"
  printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n' \
    "$MYSQL_HOST" "${MYSQL_PORT:-3306}" "$MYSQL_USERNAME" "$MYSQL_PASSWORD" >"$MYSQL_DEFAULTS"
}

mysql_query() {
  local sql="$1"
  local output
  if ! output="$("$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw "$MYSQL_DBNAME" -e "$sql" 2>"$ERROR_PATH")"; then
    fail "read-only metadata query failed"
    return 1
  fi
  printf '%s' "$output"
}

mysql_query_optional() {
  local sql="$1"
  "$MYSQL_BIN" --defaults-extra-file="$MYSQL_DEFAULTS" --batch --skip-column-names --raw "$MYSQL_DBNAME" -e "$sql" 2>"$ERROR_PATH"
}

table_present() {
  local table="$1"
  mysql_query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '$table';"
}

emit_metadata() {
  local server
  server="$(mysql_query "SELECT VERSION(), REPLACE(@@version_comment, CHAR(9), ' '), DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ');")"
  local version engine database_time
  IFS=$'\t' read -r version engine database_time <<<"$server"
  engine="${engine//$'\n'/ }"

  printf 'metadata\tenvironment\t%s\n' "$ENVIRONMENT"
  printf 'metadata\tcollected_at_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'metadata\tdatabase_time_utc\t%s\n' "$database_time"
  printf 'metadata\tcommit_sha\t%s\n' "$COMMIT_SHA"
  printf 'metadata\timage_sha\t%s\n' "$IMAGE_SHA"
  printf 'server\tengine\t%s\n' "$engine"
  printf 'server\tversion\t%s\n' "$version"
}

emit_migration_state() {
  local present state version dirty rows
  present="$(table_present schema_migrations)"
  MIGRATION_PRESENT="$present"
  if [ "$present" != "1" ]; then
    printf 'migration\tschema_migrations\tabsent\tversion=unknown\tdirty=unknown\n'
    return
  fi
  state="$(mysql_query "SELECT COALESCE(MAX(version), 0), COALESCE(MAX(dirty + 0), 0), COUNT(*) FROM schema_migrations;")"
  IFS=$'\t' read -r version dirty rows <<<"$state"
  MIGRATION_VERSION="$version"
  MIGRATION_DIRTY="$dirty"
  printf 'migration\tschema_migrations\tpresent\tversion=%s\tdirty=%s\trows=%s\n' "$version" "$dirty" "$rows"
}

emit_candidate_tables() {
  local table metadata present estimated_rows size_bytes updated_at
  for table in $CANDIDATE_TABLES; do
    metadata="$(mysql_query "SELECT COUNT(*), COALESCE(MAX(TABLE_ROWS), 0), COALESCE(MAX(DATA_LENGTH + INDEX_LENGTH), 0), COALESCE(DATE_FORMAT(MAX(UPDATE_TIME), '%Y-%m-%dT%H:%i:%sZ'), 'unknown') FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '$table';")"
    IFS=$'\t' read -r present estimated_rows size_bytes updated_at <<<"$metadata"
    printf 'candidate_table\t%s\tpresent=%s\testimated_rows=%s\tsize_bytes=%s\tupdate_time=%s\n' \
      "$table" "$present" "$estimated_rows" "$size_bytes" "$updated_at"
  done
}

emit_exact_row_counts() {
  local table present exact_rows
  for table in $CANDIDATE_TABLES; do
    present="$(table_present "$table")"
    if [ "$present" != "1" ]; then
      printf 'exact_rows\t%s\tstate=absent\n' "$table"
      continue
    fi
    exact_rows="$(mysql_query "SELECT COUNT(*) FROM $table;")"
    printf 'exact_rows\t%s\tstate=available\trows=%s\n' "$table" "$exact_rows"
  done
}

emit_schema_signatures() {
  local table present columns indexes column_sha index_sha signature
  for table in $CANDIDATE_TABLES; do
    present="$(table_present "$table")"
    if [ "$present" != "1" ]; then
      printf 'schema_signature\t%s\tstate=absent\n' "$table"
      continue
    fi
    signature="$(mysql_query "SELECT
      COUNT(*),
      COALESCE(SHA2(GROUP_CONCAT(CONCAT_WS(':', ORDINAL_POSITION, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, '<NULL>'), EXTRA) ORDER BY ORDINAL_POSITION SEPARATOR '|'), 256), 'unavailable')
      FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '$table';")"
    IFS=$'\t' read -r columns column_sha <<<"$signature"
    signature="$(mysql_query "SELECT
      COUNT(*),
      COALESCE(SHA2(GROUP_CONCAT(CONCAT_WS(':', INDEX_NAME, SEQ_IN_INDEX, COLUMN_NAME, NON_UNIQUE, COALESCE(SUB_PART, 0)) ORDER BY INDEX_NAME, SEQ_IN_INDEX SEPARATOR '|'), 256), 'unavailable')
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '$table';")"
    IFS=$'\t' read -r indexes index_sha <<<"$signature"
    printf 'schema_signature\t%s\tstate=available\tcolumns=%s\tindexes=%s\tcolumn_sha256=%s\tindex_sha256=%s\n' \
      "$table" "$columns" "$indexes" "$column_sha" "$index_sha"
  done
}

emit_performance_schema_io() {
  local enabled uptime table io count_star timer_wait reads writes
  enabled="$(mysql_query "SELECT @@performance_schema + 0;")"
  PERFORMANCE_ENABLED="$enabled"
  if [ "$enabled" != "1" ]; then
    printf 'performance_schema\tstate=disabled\tcounter_scope=unavailable\n'
    return
  fi

  if uptime="$(mysql_query_optional "SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Uptime';")" && [ -n "$uptime" ]; then
    :
  else
    uptime="unknown"
  fi
  printf 'performance_schema\tstate=enabled\tcounter_scope=since_server_start_or_last_summary_truncate\tuptime_seconds=%s\n' "$uptime"
  printf 'performance_schema\tzero_io_interpretation=not_proof_without_full_observation_window\n'
  for table in $CANDIDATE_TABLES; do
    if io="$(mysql_query_optional "SELECT COALESCE(SUM(COUNT_STAR), 0), COALESCE(SUM(SUM_TIMER_WAIT), 0), COALESCE(SUM(COUNT_READ), 0), COALESCE(SUM(COUNT_WRITE), 0) FROM performance_schema.table_io_waits_summary_by_table WHERE OBJECT_SCHEMA = DATABASE() AND OBJECT_NAME = '$table';")" && [ -n "$io" ]; then
      IFS=$'\t' read -r count_star timer_wait reads writes <<<"$io"
      printf '%s\tavailable\t%s\t%s\n' "$table" "$reads" "$writes" >>"$IO_CACHE_PATH"
      printf 'table_io\t%s\tcount_star=%s\ttimer_wait=%s\treads=%s\twrites=%s\n' "$table" "$count_star" "$timer_wait" "$reads" "$writes"
    else
      printf '%s\tunavailable\tunknown\tunknown\n' "$table" >>"$IO_CACHE_PATH"
      printf 'table_io\t%s\tstate=unavailable\n' "$table"
    fi
  done
}

emit_dependencies() {
  local table dependencies fk triggers view_refs opaque_views routine_refs opaque_routines event_refs opaque_events
  for table in $CANDIDATE_TABLES; do
    dependencies="$(mysql_query "SELECT
      (SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE (TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '$table' AND REFERENCED_TABLE_NAME IS NOT NULL) OR (REFERENCED_TABLE_SCHEMA = DATABASE() AND REFERENCED_TABLE_NAME = '$table')),
      (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = DATABASE() AND (EVENT_OBJECT_TABLE = '$table' OR INSTR(LOWER(ACTION_STATEMENT), '$table') > 0)),
      (SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = DATABASE() AND VIEW_DEFINITION IS NOT NULL AND INSTR(LOWER(VIEW_DEFINITION), '$table') > 0),
      (SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = DATABASE() AND VIEW_DEFINITION IS NULL),
      (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = DATABASE() AND ROUTINE_DEFINITION IS NOT NULL AND INSTR(LOWER(ROUTINE_DEFINITION), '$table') > 0),
      (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = DATABASE() AND ROUTINE_DEFINITION IS NULL),
      (SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = DATABASE() AND EVENT_DEFINITION IS NOT NULL AND INSTR(LOWER(EVENT_DEFINITION), '$table') > 0),
      (SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = DATABASE() AND EVENT_DEFINITION IS NULL);")"
    IFS=$'\t' read -r fk triggers view_refs opaque_views routine_refs opaque_routines event_refs opaque_events <<<"$dependencies"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$table" "$fk" "$triggers" "$view_refs" "$opaque_views" "$routine_refs" "$opaque_routines" "$event_refs" "$opaque_events" \
      >>"$DEPENDENCY_CACHE_PATH"
    printf 'dependency\t%s\tfk=%s\ttriggers=%s\tviews=%s\topaque_views=%s\troutines=%s\topaque_routines=%s\tevents=%s\topaque_events=%s\n' \
      "$table" "$fk" "$triggers" "$view_refs" "$opaque_views" "$routine_refs" "$opaque_routines" "$event_refs" "$opaque_events"
  done
}

emit_parity_result() {
  local name="$1"
  local required_tables="$2"
  local sql="$3"
  local table present result
  for table in $required_tables; do
    present="$(table_present "$table")"
    if [ "$present" != "1" ]; then
      printf '%s\tnot_applicable\tmissing_table=%s\n' "$name" "$table" >>"$PARITY_CACHE_PATH"
      printf 'parity\t%s\tstate=not_applicable\tmissing_table=%s\n' "$name" "$table"
      return
    fi
  done
  if result="$(mysql_query_optional "$sql")" && [ -n "$result" ]; then
    printf '%s\tavailable\t%s\n' "$name" "$result" >>"$PARITY_CACHE_PATH"
    printf 'parity\t%s\tstate=available\t%s\n' "$name" "$result"
  else
    printf '%s\tunavailable\treason=schema_or_privilege_mismatch\n' "$name" >>"$PARITY_CACHE_PATH"
    printf 'parity\t%s\tstate=unavailable\treason=schema_or_privilege_mismatch\n' "$name"
  fi
}

emit_parity_summaries() {
  if [ "$RETIREMENT_SCOPE" = "all" ] || [ "$RETIREMENT_SCOPE" = "identity" ]; then
    emit_identity_schema_contracts
    emit_parity_result children_to_profiles "children profiles" "SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('mapped_rows=', COALESCE(SUM(p.id IS NOT NULL), 0)),
    CONCAT('missing_rows=', COALESCE(SUM(p.id IS NULL), 0)),
    CONCAT('mismatched_rows=', COALESCE(SUM(p.id IS NOT NULL AND NOT (
      p.name <=> c.name AND p.id_card <=> c.id_card AND p.gender <=> c.gender AND
      p.birthday <=> c.birthday AND p.height <=> c.height AND p.weight <=> c.weight AND
      p.deleted_at <=> c.deleted_at AND p.version <=> c.version)), 0))
    FROM children c LEFT JOIN profiles p ON p.id = c.id;"

    emit_parity_result guardianships_to_profile_links "guardianships profile_links" "SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('mapped_rows=', COALESCE(SUM(p.id IS NOT NULL), 0)),
    CONCAT('missing_rows=', COALESCE(SUM(p.id IS NULL), 0)),
    CONCAT('mismatched_rows=', COALESCE(SUM(p.id IS NOT NULL AND NOT (
      p.user_id <=> g.user_id AND p.profile_id <=> g.child_id AND
      p.type <=> IF(LOWER(TRIM(g.relation)) = 'self', 'self', 'relation') AND
      p.relation <=> CASE LOWER(TRIM(g.relation)) WHEN 'self' THEN 'self' WHEN 'parent' THEN 'parent' WHEN 'grandparent' THEN 'grandparent' ELSE 'other' END AND
      p.revoked_at <=> g.revoked_at AND p.deleted_at <=> g.deleted_at AND p.version <=> g.version)), 0))
    FROM guardianships g LEFT JOIN profile_links p ON p.id = g.id;"
  fi

  if [ "$RETIREMENT_SCOPE" = "all" ] || [ "$RETIREMENT_SCOPE" = "authn" ]; then
    emit_parity_result auth_accounts_to_login_identities "auth_accounts auth_login_identities" "WITH account_facts AS (
    SELECT a.*,
      a.type IN ('opera', 'mock-consumer', 'wc-minip', 'wc-com') AS supported,
      (TRIM(a.external_id) <> '' AND (a.type NOT IN ('wc-minip', 'wc-com') OR TRIM(COALESCE(a.app_id, '')) <> '')) AS valid_key,
      (SELECT COUNT(*) FROM auth_login_identities li
       WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) AS UNSIGNED) = a.id
         AND CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_table')) AS BINARY) = CAST('auth_accounts' AS BINARY)) AS mapping_count,
      EXISTS (
        SELECT 1 FROM auth_login_identities li
        WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) AS UNSIGNED) = a.id
          AND CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_table')) AS BINARY) = CAST('auth_accounts' AS BINARY)
          AND li.user_id = a.user_id
          AND CAST(li.provider AS BINARY) = CAST(CASE a.type WHEN 'wc-minip' THEN 'wechat_minip' WHEN 'wc-com' THEN 'wecom' ELSE 'username' END AS BINARY)
          AND CAST(li.realm AS BINARY) = CAST(CASE WHEN a.type = 'opera' AND a.scoped_tenant_id <> 0 THEN CAST(a.scoped_tenant_id AS CHAR)
                              WHEN a.type IN ('wc-minip', 'wc-com') THEN TRIM(a.app_id) ELSE 'default' END AS BINARY)
          AND CAST(li.identifier AS BINARY) = CAST(TRIM(a.external_id) AS BINARY)
          AND CAST(li.global_identifier AS BINARY) <=> CAST(NULLIF(TRIM(COALESCE(a.unique_id, '')), '') AS BINARY)
          AND CAST(li.status AS BINARY) = CAST(CASE a.status WHEN 1 THEN 'active' WHEN 2 THEN 'archived' WHEN 3 THEN 'deleted' ELSE 'disabled' END AS BINARY)
      ) AS exact_mapping
    FROM auth_accounts a
  )
  SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('supported_rows=', COALESCE(SUM(supported), 0)),
    CONCAT('valid_supported_rows=', COALESCE(SUM(supported AND valid_key), 0)),
    CONCAT('invalid_supported_rows=', COALESCE(SUM(supported AND NOT valid_key), 0)),
    CONCAT('unsupported_rows=', COALESCE(SUM(NOT supported), 0)),
    CONCAT('mapped_rows=', COALESCE(SUM(supported AND valid_key AND mapping_count > 0), 0)),
    CONCAT('missing_supported_rows=', COALESCE(SUM(supported AND valid_key AND mapping_count = 0), 0)),
    CONCAT('mismatched_mapped_rows=', COALESCE(SUM(supported AND valid_key AND mapping_count > 0 AND NOT exact_mapping), 0)),
    CONCAT('duplicate_mapped_rows=', COALESCE(SUM(supported AND valid_key AND mapping_count > 1), 0))
  FROM account_facts;"

    emit_parity_result legacy_credentials_to_authn "auth_credentials_legacy auth_credentials auth_login_identities auth_accounts" "WITH credential_facts AS (
    SELECT lc.*,
      ((lc.type = 'password' OR COALESCE(lc.idp, '') = '')
        AND COALESCE(OCTET_LENGTH(lc.material), 0) > 0 AND COALESCE(lc.algo, '') <> '') AS password_eligible,
      (lc.type = 'password'
        AND (COALESCE(OCTET_LENGTH(lc.material), 0) = 0 OR COALESCE(lc.algo, '') = '')) AS invalid_password,
      (lc.type = 'phone_otp' OR COALESCE(lc.idp, '') = 'phone') AS phone_artifact,
      (lc.type IN ('oauth_wx_minip', 'oauth_wx_open', 'oauth_wx_scan', 'oauth_wecom')) AS oauth_artifact,
      (SELECT COUNT(*) FROM auth_credentials c
       WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(c.params_json, '$.legacy_credential_id')) AS UNSIGNED) = lc.id) AS password_mapping_count,
      EXISTS (
        SELECT 1 FROM auth_credentials c
        JOIN auth_login_identities li ON li.id = c.login_identity_id
        WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(c.params_json, '$.legacy_credential_id')) AS UNSIGNED) = lc.id
          AND CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) AS UNSIGNED) = lc.account_id
          AND SHA2(c.material, 256) <=> SHA2(lc.material, 256)
          AND c.algo <=> lc.algo
          AND c.status = IF(lc.status = 1, 'enabled', 'disabled')
          AND c.failed_attempts <=> lc.failed_attempts
          AND c.locked_until <=> lc.locked_until
          AND c.last_success_at <=> lc.last_success_at
          AND c.last_failure_at <=> lc.last_failure_at
      ) AS exact_password_mapping,
      (SELECT COUNT(*) FROM auth_login_identities li
       WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_credential_id')) AS UNSIGNED) = lc.id) AS phone_mapping_count,
      EXISTS (
        SELECT 1 FROM auth_login_identities li
        JOIN auth_accounts a ON a.id = lc.account_id
        WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_credential_id')) AS UNSIGNED) = lc.id
          AND li.user_id = a.user_id
          AND li.provider = 'phone'
          AND li.realm = 'global'
          AND li.identifier = TRIM(lc.idp_identifier)
          AND li.status = CASE
            WHEN a.status = 1 AND lc.status <> 1 THEN 'disabled'
            WHEN a.status = 1 THEN 'active'
            WHEN a.status = 2 THEN 'archived'
            WHEN a.status = 3 THEN 'deleted'
            ELSE 'disabled'
          END
      ) AS exact_phone_mapping,
      EXISTS (SELECT 1 FROM auth_accounts a WHERE a.id = lc.account_id) AS account_exists,
      EXISTS (
        SELECT 1 FROM auth_login_identities li
        WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) AS UNSIGNED) = lc.account_id
          AND li.provider = CASE lc.type
            WHEN 'oauth_wx_minip' THEN 'wechat_minip'
            WHEN 'oauth_wecom' THEN 'wecom'
            ELSE 'wechat_open'
          END
      ) AS oauth_identity_exists
    FROM auth_credentials_legacy lc
  )
  SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('password_eligible_rows=', COALESCE(SUM(password_eligible), 0)),
    CONCAT('password_mapped_rows=', COALESCE(SUM(password_eligible AND password_mapping_count > 0), 0)),
    CONCAT('password_unmapped_rows=', COALESCE(SUM(password_eligible AND password_mapping_count = 0), 0)),
    CONCAT('password_material_mismatches=', COALESCE(SUM(password_eligible AND password_mapping_count > 0 AND NOT exact_password_mapping), 0)),
    CONCAT('password_duplicate_mappings=', COALESCE(SUM(password_eligible AND password_mapping_count > 1), 0)),
    CONCAT('invalid_password_rows=', COALESCE(SUM(invalid_password), 0)),
    CONCAT('phone_eligible_rows=', COALESCE(SUM(phone_artifact AND TRIM(COALESCE(idp_identifier, '')) <> '' AND account_exists), 0)),
    CONCAT('phone_identity_mapped_rows=', COALESCE(SUM(phone_artifact AND TRIM(COALESCE(idp_identifier, '')) <> '' AND account_exists AND phone_mapping_count > 0), 0)),
    CONCAT('phone_blank_identifier_rows=', COALESCE(SUM(phone_artifact AND TRIM(COALESCE(idp_identifier, '')) = ''), 0)),
    CONCAT('phone_orphan_account_rows=', COALESCE(SUM(phone_artifact AND NOT account_exists), 0)),
    CONCAT('phone_identity_mismatches=', COALESCE(SUM(phone_artifact AND phone_mapping_count > 0 AND NOT exact_phone_mapping), 0)),
    CONCAT('phone_duplicate_mappings=', COALESCE(SUM(phone_artifact AND phone_mapping_count > 1), 0)),
    CONCAT('oauth_artifact_rows=', COALESCE(SUM(oauth_artifact), 0)),
    CONCAT('oauth_redundant_rows=', COALESCE(SUM(oauth_artifact AND oauth_identity_exists), 0)),
    CONCAT('oauth_unmapped_rows=', COALESCE(SUM(oauth_artifact AND NOT oauth_identity_exists), 0)),
    CONCAT('unknown_credential_rows=', COALESCE(SUM(NOT password_eligible AND NOT invalid_password AND NOT phone_artifact AND NOT oauth_artifact), 0))
  FROM credential_facts;"
  fi
}

cached_parity_state() {
  local name="$1"
  awk -F '\t' -v wanted="$name" '$1 == wanted { print $2; exit }' "$PARITY_CACHE_PATH"
}

cached_parity_value() {
  local name="$1"
  local key="$2"
  awk -F '\t' -v wanted="$name" -v key="$key" '
    $1 == wanted {
      for (i = 3; i <= NF; i++) {
        split($i, pair, "=")
        if (pair[1] == key) {
          print pair[2]
          exit
        }
      }
    }
  ' "$PARITY_CACHE_PATH"
}

common_retirement_blocker() {
  local table="$1"
  local io_line io_state reads writes dependency_line dependency_total value
  if [ "$MIGRATION_PRESENT" != "1" ]; then
    printf 'schema_migrations_absent'
    return
  fi
  if [ "$MIGRATION_DIRTY" != "0" ]; then
    printf 'schema_migrations_dirty'
    return
  fi
  if ! [[ "$MIGRATION_VERSION" =~ ^[0-9]+$ ]] || [ "$MIGRATION_VERSION" -lt 19 ]; then
    printf 'migration_version_before_19'
    return
  fi
  if [ "$PERFORMANCE_ENABLED" != "1" ]; then
    if ! owner_io_waiver_allows "$table"; then
      printf 'performance_schema_unavailable'
      return
    fi
  else
    io_line="$(awk -F '\t' -v wanted="$table" '$1 == wanted { print; exit }' "$IO_CACHE_PATH")"
    if [ -z "$io_line" ]; then
      if ! owner_io_waiver_allows "$table"; then
        printf 'table_io_unavailable'
        return
      fi
    else
      IFS=$'\t' read -r _ io_state reads writes <<<"$io_line"
      if [ "$io_state" != "available" ]; then
        if ! owner_io_waiver_allows "$table"; then
          printf 'table_io_unavailable'
          return
        fi
      elif [ "$reads" != "0" ] || [ "$writes" != "0" ]; then
        printf 'instantaneous_io_nonzero'
        return
      fi
    fi
  fi
  dependency_line="$(awk -F '\t' -v wanted="$table" '$1 == wanted { print; exit }' "$DEPENDENCY_CACHE_PATH")"
  if [ -z "$dependency_line" ]; then
    printf 'dependency_evidence_unavailable'
    return
  fi
  dependency_total=0
  while IFS= read -r value; do
    if ! [[ "$value" =~ ^[0-9]+$ ]]; then
      printf 'dependency_evidence_unavailable'
      return
    fi
    dependency_total=$((dependency_total + value))
  done < <(printf '%s\n' "$dependency_line" | awk -F '\t' '{ for (i = 2; i <= NF; i++) print $i }')
  if [ "$dependency_total" -ne 0 ]; then
    printf 'database_dependencies_present'
    return
  fi
}

owner_io_waiver_allows() {
  local table="$1"
  [ "$OWNER_IO_WAIVER" = "platform_tables" ] \
    && [[ " schema_version tenants data_dictionary " == *" $table "* ]]
}

io_evidence_mode() {
  local table="$1"
  local io_line io_state
  if ! owner_io_waiver_allows "$table"; then
    printf 'instantaneous'
    return
  fi
  if [ "$PERFORMANCE_ENABLED" != "1" ]; then
    printf 'owner_io_waiver'
    return
  fi
  io_line="$(awk -F '\t' -v wanted="$table" '$1 == wanted { print; exit }' "$IO_CACHE_PATH")"
  if [ -z "$io_line" ]; then
    printf 'owner_io_waiver'
    return
  fi
  IFS=$'\t' read -r _ io_state _ _ <<<"$io_line"
  if [ "$io_state" != "available" ]; then
    printf 'owner_io_waiver'
  else
    printf 'instantaneous'
  fi
}

emit_simple_eligibility() {
  local table="$1"
  local repository_gate="$2"
  local present blocker evidence
  present="$(table_present "$table")"
  if [ "$present" != "1" ]; then
    printf 'eligibility\t%s\tstate=already_absent\tevidence=instantaneous\n' "$table"
    return
  fi
  blocker="$(common_retirement_blocker "$table")"
  if [ -n "$blocker" ]; then
    printf 'eligibility\t%s\tstate=blocked\treason=%s\tevidence=instantaneous\n' "$table" "$blocker"
    return
  fi
  evidence="$(io_evidence_mode "$table")"
  printf 'eligibility\t%s\tstate=eligible\trepository_gate=%s\tevidence=%s\n' \
    "$table" "$repository_gate" "$evidence"
}

emit_auth_account_eligibility() {
  local present blocker state key value
  present="$(table_present auth_accounts)"
  if [ "$present" != "1" ]; then
    printf 'eligibility\tauth_accounts\tstate=already_absent\tevidence=instantaneous\n'
    return
  fi
  blocker="$(common_retirement_blocker auth_accounts)"
  if [ -n "$blocker" ]; then
    printf 'eligibility\tauth_accounts\tstate=blocked\treason=%s\tevidence=instantaneous\n' "$blocker"
    return
  fi
  state="$(cached_parity_state auth_accounts_to_login_identities)"
  if [ "$state" != "available" ]; then
    printf 'eligibility\tauth_accounts\tstate=blocked\treason=account_parity_unavailable\tevidence=instantaneous\n'
    return
  fi
  for key in invalid_supported_rows unsupported_rows missing_supported_rows mismatched_mapped_rows duplicate_mapped_rows; do
    value="$(cached_parity_value auth_accounts_to_login_identities "$key")"
    if [ -z "$value" ] || [ "$value" != "0" ]; then
      printf 'eligibility\tauth_accounts\tstate=blocked\treason=account_parity_%s\tevidence=instantaneous\n' "$key"
      return
    fi
  done
  printf 'eligibility\tauth_accounts\tstate=eligible\trepository_gate=authn_retirement_migration\tevidence=instantaneous\n'
}

emit_auth_credential_eligibility() {
  local present old_shape blocker state key value eligible mapped
  present="$(table_present auth_credentials_legacy)"
  if [ "$present" != "1" ]; then
    old_shape="$(mysql_query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_credentials' AND COLUMN_NAME = 'account_id';")"
    if [ "$old_shape" != "0" ]; then
      printf 'eligibility\tauth_credentials_legacy\tstate=blocked\treason=old_shaped_auth_credentials_present\tevidence=instantaneous\n'
    else
      printf 'eligibility\tauth_credentials_legacy\tstate=already_absent\tevidence=instantaneous\n'
    fi
    return
  fi
  blocker="$(common_retirement_blocker auth_credentials_legacy)"
  if [ -n "$blocker" ]; then
    printf 'eligibility\tauth_credentials_legacy\tstate=blocked\treason=%s\tevidence=instantaneous\n' "$blocker"
    return
  fi
  state="$(cached_parity_state legacy_credentials_to_authn)"
  if [ "$state" != "available" ]; then
    printf 'eligibility\tauth_credentials_legacy\tstate=blocked\treason=credential_parity_unavailable\tevidence=instantaneous\n'
    return
  fi
  for key in password_unmapped_rows password_material_mismatches password_duplicate_mappings invalid_password_rows phone_blank_identifier_rows phone_orphan_account_rows phone_identity_mismatches phone_duplicate_mappings oauth_unmapped_rows unknown_credential_rows; do
    value="$(cached_parity_value legacy_credentials_to_authn "$key")"
    if [ -z "$value" ] || [ "$value" != "0" ]; then
      printf 'eligibility\tauth_credentials_legacy\tstate=blocked\treason=credential_parity_%s\tevidence=instantaneous\n' "$key"
      return
    fi
  done
  for key in password phone oauth; do
    case "$key" in
      password)
        eligible="$(cached_parity_value legacy_credentials_to_authn password_eligible_rows)"
        mapped="$(cached_parity_value legacy_credentials_to_authn password_mapped_rows)"
        ;;
      phone)
        eligible="$(cached_parity_value legacy_credentials_to_authn phone_eligible_rows)"
        mapped="$(cached_parity_value legacy_credentials_to_authn phone_identity_mapped_rows)"
        ;;
      oauth)
        eligible="$(cached_parity_value legacy_credentials_to_authn oauth_artifact_rows)"
        mapped="$(cached_parity_value legacy_credentials_to_authn oauth_redundant_rows)"
        ;;
    esac
    if [ -z "$eligible" ] || [ -z "$mapped" ] || [ "$eligible" != "$mapped" ]; then
      printf 'eligibility\tauth_credentials_legacy\tstate=blocked\treason=credential_%s_count_mismatch\tevidence=instantaneous\n' "$key"
      return
    fi
  done
  printf 'eligibility\tauth_credentials_legacy\tstate=eligible\trepository_gate=authn_retirement_migration\tevidence=instantaneous\n'
}

emit_eligibility() {
  local table present
  for table in children guardianships; do
    present="$(table_present "$table")"
    if [ "$present" = "1" ]; then
      printf 'eligibility\t%s\tstate=blocked\treason=requires_000019_execution\tevidence=instantaneous\n' "$table"
    else
      printf 'eligibility\t%s\tstate=already_absent\tevidence=instantaneous\n' "$table"
    fi
  done
  if [ "$RETIREMENT_SCOPE" = "all" ] || [ "$RETIREMENT_SCOPE" = "schema_version" ]; then
    emit_simple_eligibility schema_version retire_schema_version
  else
    printf 'eligibility\tschema_version\tstate=not_evaluated\treason=scope_excluded\n'
  fi
  if [ "$RETIREMENT_SCOPE" = "all" ] || [ "$RETIREMENT_SCOPE" = "platform" ]; then
    emit_simple_eligibility tenants remove_bootstrap_reference
    emit_simple_eligibility data_dictionary remove_bootstrap_reference
  else
    printf 'eligibility\ttenants\tstate=not_evaluated\treason=scope_excluded\n'
    printf 'eligibility\tdata_dictionary\tstate=not_evaluated\treason=scope_excluded\n'
  fi
  if [ "$RETIREMENT_SCOPE" = "all" ] || [ "$RETIREMENT_SCOPE" = "authn" ]; then
    emit_auth_account_eligibility
    emit_auth_credential_eligibility
  else
    printf 'eligibility\tauth_accounts\tstate=not_evaluated\treason=scope_excluded\n'
    printf 'eligibility\tauth_credentials_legacy\tstate=not_evaluated\treason=scope_excluded\n'
  fi
  for table in operation_logs audit_logs auth_token_audit; do
    present="$(table_present "$table")"
    if [ "$present" = "1" ]; then
      printf 'eligibility\t%s\tstate=deferred\treason=product_or_compliance_decision\tevidence=instantaneous\n' "$table"
    else
      printf 'eligibility\t%s\tstate=already_absent\tevidence=instantaneous\n' "$table"
    fi
  done
}

main() {
  umask 077
  validate_configuration || return 1
  prepare_container_client_fallback || return 1
  require_mysql8_client || return 1
  prepare_defaults_file
  ERROR_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-error.XXXXXX")"
  IO_CACHE_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-io.XXXXXX")"
  DEPENDENCY_CACHE_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-dependencies.XXXXXX")"
  PARITY_CACHE_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-parity.XXXXXX")"
  chmod 0600 "$ERROR_PATH"
  chmod 0600 "$IO_CACHE_PATH" "$DEPENDENCY_CACHE_PATH" "$PARITY_CACHE_PATH"

  printf 'legacy_retirement_preflight\tformat_version=2\tquery_mode=read_only_aggregate\tscope=%s\towner_io_waiver=%s\n' \
    "$RETIREMENT_SCOPE" "$OWNER_IO_WAIVER"
  emit_metadata
  emit_migration_state
  emit_candidate_tables
  emit_performance_schema_io
  emit_dependencies
  emit_exact_row_counts
  emit_schema_signatures
  emit_parity_summaries
  emit_eligibility
  printf 'legacy_retirement_preflight\tresult=success\n'
}

main
