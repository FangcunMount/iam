#!/usr/bin/env bash

set -euo pipefail

MYSQL_BIN="${IAM_RETIREMENT_MYSQL_BIN:-mysql}"
ENVIRONMENT="${IAM_RETIREMENT_ENVIRONMENT:-}"
COMMIT_SHA="${IAM_RETIREMENT_COMMIT_SHA:-}"
IMAGE_SHA="${IAM_RETIREMENT_IMAGE_SHA:-unknown}"
SUPPLIED_DEFAULTS="${IAM_RETIREMENT_MYSQL_DEFAULTS_FILE:-}"

MYSQL_DEFAULTS=""
ERROR_PATH=""
OWNS_DEFAULTS=0

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
  if [ "$present" != "1" ]; then
    printf 'migration\tschema_migrations\tabsent\tversion=unknown\tdirty=unknown\n'
    return
  fi
  state="$(mysql_query "SELECT COALESCE(MAX(version), 0), COALESCE(MAX(dirty + 0), 0), COUNT(*) FROM schema_migrations;")"
  IFS=$'\t' read -r version dirty rows <<<"$state"
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

emit_performance_schema_io() {
  local enabled uptime table io count_star timer_wait reads writes
  enabled="$(mysql_query "SELECT @@performance_schema + 0;")"
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
      printf 'table_io\t%s\tcount_star=%s\ttimer_wait=%s\treads=%s\twrites=%s\n' "$table" "$count_star" "$timer_wait" "$reads" "$writes"
    else
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
      printf 'parity\t%s\tstate=not_applicable\tmissing_table=%s\n' "$name" "$table"
      return
    fi
  done
  if result="$(mysql_query_optional "$sql")" && [ -n "$result" ]; then
    printf 'parity\t%s\tstate=available\t%s\n' "$name" "$result"
  else
    printf 'parity\t%s\tstate=unavailable\treason=schema_or_privilege_mismatch\n' "$name"
  fi
}

emit_parity_summaries() {
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

  emit_parity_result auth_accounts_to_login_identities "auth_accounts auth_login_identities" "SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('supported_rows=', COALESCE(SUM(a.type IN ('opera', 'mock-consumer', 'wc-minip', 'wc-com')), 0)),
    CONCAT('mapped_rows=', COALESCE(SUM(a.type IN ('opera', 'mock-consumer', 'wc-minip', 'wc-com') AND EXISTS (
      SELECT 1 FROM auth_login_identities li
      WHERE JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) = CAST(a.id AS CHAR))), 0)),
    CONCAT('missing_supported_rows=', COALESCE(SUM(a.type IN ('opera', 'mock-consumer', 'wc-minip', 'wc-com') AND NOT EXISTS (
      SELECT 1 FROM auth_login_identities li
      WHERE JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) = CAST(a.id AS CHAR))), 0)),
    CONCAT('mismatched_mapped_rows=', COALESCE(SUM(a.type IN ('opera', 'mock-consumer', 'wc-minip', 'wc-com') AND EXISTS (
      SELECT 1 FROM auth_login_identities li
      WHERE JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) = CAST(a.id AS CHAR))
      AND NOT EXISTS (
        SELECT 1 FROM auth_login_identities li
        WHERE JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_account_id')) = CAST(a.id AS CHAR)
          AND li.user_id = a.user_id
          AND li.provider = CASE a.type WHEN 'wc-minip' THEN 'wechat_minip' WHEN 'wc-com' THEN 'wecom' ELSE 'username' END
          AND li.realm = CASE WHEN a.type = 'opera' AND a.scoped_tenant_id <> 0 THEN CAST(a.scoped_tenant_id AS CHAR)
                              WHEN a.type IN ('wc-minip', 'wc-com') THEN TRIM(a.app_id) ELSE 'default' END
          AND li.identifier = TRIM(a.external_id)
          AND li.status = CASE a.status WHEN 1 THEN 'active' WHEN 2 THEN 'archived' WHEN 3 THEN 'deleted' ELSE 'disabled' END)), 0))
    FROM auth_accounts a;"

  emit_parity_result legacy_credentials_to_authn "auth_credentials_legacy auth_credentials auth_login_identities" "SELECT
    CONCAT('legacy_rows=', COUNT(*)),
    CONCAT('password_eligible_rows=', COALESCE(SUM((lc.type = 'password' OR COALESCE(lc.idp, '') = '') AND OCTET_LENGTH(lc.material) > 0 AND COALESCE(lc.algo, '') <> ''), 0)),
    CONCAT('password_mapped_rows=', COALESCE(SUM(
      (lc.type = 'password' OR COALESCE(lc.idp, '') = '') AND OCTET_LENGTH(lc.material) > 0 AND COALESCE(lc.algo, '') <> ''
      AND EXISTS (SELECT 1 FROM auth_credentials c
                  WHERE JSON_UNQUOTE(JSON_EXTRACT(c.params_json, '$.legacy_credential_id')) = CAST(lc.id AS CHAR))), 0)),
    CONCAT('password_material_mismatches=', COALESCE(SUM(
      (lc.type = 'password' OR COALESCE(lc.idp, '') = '') AND OCTET_LENGTH(lc.material) > 0 AND COALESCE(lc.algo, '') <> ''
      AND EXISTS (SELECT 1 FROM auth_credentials c
                  WHERE JSON_UNQUOTE(JSON_EXTRACT(c.params_json, '$.legacy_credential_id')) = CAST(lc.id AS CHAR))
      AND NOT EXISTS (SELECT 1 FROM auth_credentials c
                      WHERE JSON_UNQUOTE(JSON_EXTRACT(c.params_json, '$.legacy_credential_id')) = CAST(lc.id AS CHAR)
                        AND SHA2(c.material, 256) <=> SHA2(lc.material, 256)
                        AND c.algo <=> lc.algo
                        AND c.status = IF(lc.status = 1, 'enabled', 'disabled'))), 0)),
    CONCAT('phone_eligible_rows=', COALESCE(SUM(lc.type = 'phone_otp' OR COALESCE(lc.idp, '') = 'phone'), 0)),
    CONCAT('phone_identity_mapped_rows=', COALESCE(SUM(
      (lc.type = 'phone_otp' OR COALESCE(lc.idp, '') = 'phone')
      AND EXISTS (SELECT 1 FROM auth_login_identities li
                  WHERE JSON_UNQUOTE(JSON_EXTRACT(li.meta_json, '$.legacy_credential_id')) = CAST(lc.id AS CHAR))), 0))
    FROM auth_credentials_legacy lc;"
}

main() {
  umask 077
  validate_configuration || return 1
  require_mysql8_client || return 1
  prepare_defaults_file
  ERROR_PATH="$(mktemp "${TMPDIR:-/tmp}/iam-retirement-error.XXXXXX")"
  chmod 0600 "$ERROR_PATH"

  printf 'legacy_retirement_preflight\tformat_version=1\tquery_mode=read_only_aggregate\n'
  emit_metadata
  emit_migration_state
  emit_candidate_tables
  emit_performance_schema_io
  emit_dependencies
  emit_parity_summaries
  printf 'legacy_retirement_preflight\tresult=success\n'
}

main
