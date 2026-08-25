#!/usr/bin/env bash
set -Eeuo pipefail

: "${IAM_AUTHZ_CLONE_CONFIRMATION:?IAM_AUTHZ_CLONE_CONFIRMATION is required}"
: "${IAM_AUTHZ_CLONE_VALIDATION_MODE:?IAM_AUTHZ_CLONE_VALIDATION_MODE is required}"
: "${IAM_AUTHZ_RELEASE_SHA:?IAM_AUTHZ_RELEASE_SHA is required}"
: "${IAM_AUTHZ_CLONE_BACKUP_FILE:?IAM_AUTHZ_CLONE_BACKUP_FILE is required}"
: "${IAM_AUTHZ_CLONE_MYSQL_PASSWORD:?IAM_AUTHZ_CLONE_MYSQL_PASSWORD is required}"
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"
: "${DOCKER_CONFIG:?DOCKER_CONFIG is required}"

expected_confirmation="PREFLIGHT_AUTHZ_BACKUP_CLONE"
if [ "$IAM_AUTHZ_CLONE_VALIDATION_MODE" = "full_rehearsal" ]; then
  expected_confirmation="REHEARSE_AUTHZ_BACKUP_CLONE"
elif [ "$IAM_AUTHZ_CLONE_VALIDATION_MODE" != "preflight" ]; then
  echo "invalid AuthZ backup clone validation mode" >&2
  exit 1
fi
if [ "$IAM_AUTHZ_CLONE_CONFIRMATION" != "$expected_confirmation" ]; then
  echo "invalid AuthZ backup clone confirmation" >&2
  exit 1
fi
if [ "${#IAM_AUTHZ_RELEASE_SHA}" -ne 40 ] || [ -n "${IAM_AUTHZ_RELEASE_SHA//[0-9a-f]/}" ]; then
  echo "IAM release SHA must contain exactly 40 lowercase hexadecimal characters" >&2
  exit 1
fi

# A clone run must never inherit the production connection contract. The only
# accepted credential is the clone-specific secret above; connection values are
# fixed below and passed to iam-maintenance one process at a time.
for production_variable in MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_USERNAME MYSQL_PASSWORD MYSQL_DATABASE MYSQL_DBNAME; do
  if [ -n "${!production_variable:-}" ]; then
    echo "production-style database environment is not accepted (${production_variable})" >&2
    exit 1
  fi
done

backup_name="$(basename "$IAM_AUTHZ_CLONE_BACKUP_FILE")"
if ! [[ "$backup_name" =~ ^iam_backup_[0-9]{8}_[0-9]{6}\.sql\.gz$ ]]; then
  echo "backup name is invalid" >&2
  exit 1
fi
if [ -L "$IAM_AUTHZ_CLONE_BACKUP_FILE" ] || [ ! -f "$IAM_AUTHZ_CLONE_BACKUP_FILE" ]; then
  echo "backup file is unavailable or is a symbolic link" >&2
  exit 1
fi
expected_backup_dir="${RUNNER_TEMP}/iam-authz-backup-clone-${GITHUB_RUN_ID}"
actual_backup_dir="$(cd "$(dirname "$IAM_AUTHZ_CLONE_BACKUP_FILE")" && pwd -P)"
canonical_expected_dir="$(cd "$expected_backup_dir" && pwd -P)"
if [ "$actual_backup_dir" != "$canonical_expected_dir" ]; then
  echo "backup file is outside the isolated runner directory" >&2
  exit 1
fi
gzip -t "$IAM_AUTHZ_CLONE_BACKUP_FILE"

maintenance_binary="$(pwd -P)/bin/iam-maintenance"
if [ ! -x "$maintenance_binary" ]; then
  echo "IAM maintenance binary is missing or not executable" >&2
  exit 1
fi

command -v docker >/dev/null 2>&1 || {
  echo "Docker CLI is unavailable on the Mac mini runner" >&2
  exit 1
}
docker_clone() {
  docker --config "$DOCKER_CONFIG" "$@"
}
docker_clone info >/dev/null

# Use AWS's anonymous Docker Official Images mirror. Docker Desktop on the
# headless Mac runner must not consult the interactive Docker Hub keychain.
readonly clone_image="public.ecr.aws/docker/library/mysql:8.0"
readonly clone_container="iam-authz-preflight-mysql"
readonly clone_data_volume="iam-authz-preflight-mysql-data"
readonly clone_database="iam_authz_preflight"
readonly clone_port="33306"

if ! docker_clone image inspect "$clone_image" >/dev/null 2>&1; then
  docker_clone pull "$clone_image" >/dev/null
fi
docker_clone volume inspect "$clone_data_volume" >/dev/null 2>&1 || docker_clone volume create "$clone_data_volume" >/dev/null

if docker_clone container inspect "$clone_container" >/dev/null 2>&1; then
  docker_clone rm -f "$clone_container" >/dev/null
fi

# The clone-only password is present only in the short-lived initialization
# container. Once MySQL has initialized the data volume, recreate the durable
# container without any password environment variable.
docker_clone run -d \
  --name "$clone_container" \
  --publish "127.0.0.1:${clone_port}:3306" \
  --cpus "1.0" \
  --memory "2g" \
  --memory-swap "2g" \
  --pids-limit "512" \
  --env "MYSQL_ROOT_PASSWORD=${IAM_AUTHZ_CLONE_MYSQL_PASSWORD}" \
  --env MYSQL_ROOT_HOST=% \
  --volume "${clone_data_volume}:/var/lib/mysql" \
  "$clone_image" >/dev/null

clone_mysql() {
  docker_clone exec -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
    mysql --protocol=socket -uroot --batch --skip-column-names "$@"
}

wait_for_clone_mysql() {
  local ready=0
  for _ in $(seq 1 30); do
    if docker_clone exec -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
      mysqladmin --protocol=socket -uroot ping --silent >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 2
  done
  [ "$ready" = "1" ]
}

if ! wait_for_clone_mysql; then
  docker_clone logs --tail 100 "$clone_container" >&2 || true
  echo "Mac mini clone MySQL initialization did not become ready" >&2
  exit 1
fi

docker_clone rm -f "$clone_container" >/dev/null
docker_clone run -d \
  --name "$clone_container" \
  --restart unless-stopped \
  --publish "127.0.0.1:${clone_port}:3306" \
  --cpus "1.0" \
  --memory "2g" \
  --memory-swap "2g" \
  --pids-limit "512" \
  --volume "${clone_data_volume}:/var/lib/mysql" \
  "$clone_image" >/dev/null
if ! wait_for_clone_mysql; then
  docker_clone logs --tail 100 "$clone_container" >&2 || true
  echo "persistent Mac mini clone MySQL did not become ready" >&2
  exit 1
fi

# Destructive scope is intentionally non-configurable and limited to the
# dedicated clone database. The container and data volume remain for follow-up
# investigation, while each run restores a clean copy of the selected backup.
clone_mysql -e 'DROP DATABASE IF EXISTS `iam_authz_preflight`; CREATE DATABASE `iam_authz_preflight` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
gzip -dc "$IAM_AUTHZ_CLONE_BACKUP_FILE" |
  docker_clone exec -i -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
    mysql -uroot --default-character-set=utf8mb4 "$clone_database"

schema_state="$(clone_mysql "$clone_database" -e "
  SELECT COALESCE(MAX(version), 0), COALESCE(MAX(dirty + 0), 0), COUNT(*)
  FROM schema_migrations;
")"
if [ "$schema_state" != $'25\t0\t1' ]; then
  echo "restored clone must contain exactly one clean schema migration row at version 25 (actual=${schema_state})" >&2
  exit 1
fi
legacy_table_count="$(clone_mysql "$clone_database" -e "
  SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'casbin_rule';
")"
if [ "$legacy_table_count" != "1" ]; then
  echo "restored clone does not contain the expected legacy authorization table" >&2
  exit 1
fi

run_maintenance() {
  env \
    MYSQL_HOST=127.0.0.1 \
    MYSQL_PORT="$clone_port" \
    MYSQL_USER=root \
    MYSQL_PASSWORD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" \
    MYSQL_DATABASE="$clone_database" \
    "$maintenance_binary" "$@"
}

echo "Mac mini AuthZ clone restored: backup=${backup_name} schema=25 clean=true legacy_table=true"
run_maintenance authz-cutover migrate-additive \
  --confirm=MIGRATE_AUTHZ_ADDITIVE_SCHEMA --timeout=10m --lock-timeout=30s

schema_state="$(clone_mysql "$clone_database" -e "
  SELECT COALESCE(MAX(version), 0), COALESCE(MAX(dirty + 0), 0), COUNT(*)
  FROM schema_migrations;
")"
if [ "$schema_state" != $'26\t0\t1' ]; then
  echo "clone additive migration did not finish at clean schema version 26 (actual=${schema_state})" >&2
  exit 1
fi

set +e
preflight_output="$(run_maintenance authz-cutover preflight --timeout=10m 2>&1)"
preflight_status=$?
set -e
printf '%s\n' "$preflight_output"

if [[ "$preflight_output" == *assignment_unknown_or_cross_tenant_role* ]]; then
  assignment_diagnostic="$(clone_mysql "$clone_database" -e "
    SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
      'assignment_id', CAST(assignment_row.id AS CHAR),
      'assignment_tenant', assignment_row.tenant_id,
      'subject_type', assignment_row.subject_type,
      'role_id', CAST(assignment_row.role_id AS CHAR),
      'role_exists', IF(role_row.id IS NULL, FALSE, TRUE),
      'role_active', IF(role_row.id IS NULL OR role_row.deleted_at IS NOT NULL, FALSE, TRUE),
      'role_tenant', role_row.tenant_id,
      'role_name', role_row.name,
      'expected_seed_role_name', assignment_row.expected_seed_role_name,
      'candidate_role_id', CAST(candidate_role.id AS CHAR),
      'candidate_role_active', IF(candidate_role.id IS NULL, FALSE, TRUE),
      'candidate_assignment_id', CAST(candidate_assignment.id AS CHAR),
      'exact_grouping_count', (
        SELECT COUNT(*)
        FROM casbin_rule AS grouping_rule
        WHERE grouping_rule.ptype = 'g'
          AND grouping_rule.v0 = CONCAT(assignment_row.subject_type, ':', assignment_row.subject_id)
          AND grouping_rule.v1 = CONCAT('role:', assignment_row.expected_seed_role_name)
          AND grouping_rule.v2 = assignment_row.tenant_id
      )
    )), JSON_ARRAY())
    FROM (
      SELECT assignment_fact.*,
             CASE assignment_fact.id
               WHEN 902000001 THEN 'super_admin'
               WHEN 902000002 THEN 'tenant_admin'
               WHEN 902000003 THEN 'qs:admin'
               WHEN 902000004 THEN 'tenant_admin'
               WHEN 902000005 THEN 'qs:admin'
               WHEN 902000006 THEN 'qs:content_manager'
               ELSE NULL
             END AS expected_seed_role_name
      FROM authz_assignments AS assignment_fact
    ) AS assignment_row
    LEFT JOIN authz_roles AS role_row ON role_row.id = assignment_row.role_id
    LEFT JOIN authz_roles AS candidate_role
      ON candidate_role.tenant_id = assignment_row.tenant_id
     AND candidate_role.name = assignment_row.expected_seed_role_name
     AND candidate_role.deleted_at IS NULL
    LEFT JOIN authz_assignments AS candidate_assignment
      ON candidate_assignment.subject_type = assignment_row.subject_type
     AND candidate_assignment.subject_id = assignment_row.subject_id
     AND candidate_assignment.tenant_id = assignment_row.tenant_id
     AND candidate_assignment.role_id = candidate_role.id
     AND candidate_assignment.deleted_at IS NULL
     AND candidate_assignment.id <> assignment_row.id
    WHERE assignment_row.deleted_at IS NULL
      AND (role_row.id IS NULL
        OR role_row.deleted_at IS NOT NULL
        OR NOT (role_row.tenant_id <=> assignment_row.tenant_id));
  ")"
  echo "AuthZ assignment-role diagnostic (subject IDs omitted): ${assignment_diagnostic}"
fi

if [[ "$preflight_output" =~ resource_catalog_row_invalid_([0-9]+) ]]; then
  invalid_resource_id="${BASH_REMATCH[1]}"
  resource_diagnostic="$(clone_mysql "$clone_database" -e "
    SELECT JSON_OBJECT(
      'id', CAST(resource_row.id AS CHAR),
      'key', resource_row.key,
      'key_hex', HEX(resource_row.key),
      'app_name', resource_row.app_name,
      'domain', resource_row.domain,
      'type', resource_row.type,
      'actions_json_valid', JSON_VALID(resource_row.actions),
      'actions_json_type', CASE WHEN JSON_VALID(resource_row.actions) THEN JSON_TYPE(CAST(resource_row.actions AS JSON)) ELSE NULL END,
      'actions', resource_row.actions,
      'scope_kinds', resource_row.scope_kinds,
      'attribute_schema', resource_row.attribute_schema,
      'version', resource_row.version
    )
    FROM authz_resources AS resource_row
    WHERE resource_row.id = ${invalid_resource_id} AND resource_row.deleted_at IS NULL;
  ")"
  if [ -z "$resource_diagnostic" ]; then
    echo "invalid AuthZ resource catalog row was not found in the persistent clone" >&2
  else
    echo "AuthZ resource catalog diagnostic: ${resource_diagnostic}"
  fi

  catalog_identity_diagnostic="$(clone_mysql "$clone_database" -e "
    SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
      'id', CAST(catalog_row.id AS CHAR),
      'key', catalog_row.resource_key,
      'stored_app', catalog_row.stored_app,
      'expected_app', catalog_row.expected_app,
      'stored_domain', catalog_row.stored_domain,
      'expected_domain', catalog_row.expected_domain,
      'stored_type', catalog_row.stored_type,
      'expected_type', catalog_row.expected_type,
      'segment_count', catalog_row.segment_count
    )), JSON_ARRAY())
    FROM (
      SELECT resource_row.id,
             resource_row.key AS resource_key,
             resource_row.app_name AS stored_app,
             resource_row.domain AS stored_domain,
             resource_row.type AS stored_type,
             SUBSTRING_INDEX(resource_row.key, ':', 1) AS expected_app,
             SUBSTRING_INDEX(SUBSTRING_INDEX(resource_row.key, ':', 2), ':', -1) AS expected_domain,
             SUBSTRING_INDEX(SUBSTRING_INDEX(resource_row.key, ':', 3), ':', -1) AS expected_type,
             LENGTH(resource_row.key) - LENGTH(REPLACE(resource_row.key, ':', '')) + 1 AS segment_count
      FROM authz_resources AS resource_row
      WHERE resource_row.deleted_at IS NULL
    ) AS catalog_row
    WHERE catalog_row.segment_count <> 4
       OR NOT (catalog_row.stored_app <=> catalog_row.expected_app)
       OR NOT (catalog_row.stored_domain <=> catalog_row.expected_domain)
       OR NOT (catalog_row.stored_type <=> catalog_row.expected_type);
  ")"
  echo "AuthZ resource catalog identity mismatches: ${catalog_identity_diagnostic}"

  catalog_actions_diagnostic="$(clone_mysql "$clone_database" -e "
    SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
      'id', CAST(resource_row.id AS CHAR),
      'key', resource_row.key,
      'actions', resource_row.actions,
      'json_valid', JSON_VALID(resource_row.actions)
    )), JSON_ARRAY())
    FROM authz_resources AS resource_row
    WHERE resource_row.deleted_at IS NULL
      AND CASE
        WHEN COALESCE(JSON_VALID(resource_row.actions), 0) = 0 THEN 1
        WHEN JSON_TYPE(CAST(resource_row.actions AS JSON)) <> 'ARRAY' THEN 1
        ELSE 0
      END = 1;
  ")"
  echo "AuthZ resource catalog action-shape mismatches: ${catalog_actions_diagnostic}"
fi

if [ "$IAM_AUTHZ_CLONE_VALIDATION_MODE" = "full_rehearsal" ]; then
  evidence_dir="${RUNNER_TEMP}/iam-authz-clone-evidence-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT:-1}"
  mkdir -p "$evidence_dir"
  chmod 0700 "$evidence_dir"

  run_evidence_step() {
    local filename="$1"
    shift
    run_maintenance "$@" | tee "${evidence_dir}/${filename}"
    chmod 0600 "${evidence_dir}/${filename}"
  }

  run_evidence_step 03-apply.json authz-cutover apply \
    --confirm=APPLY_AUTHZ_CUTOVER --timeout=10m --lock-timeout=30s
  run_evidence_step 04-verify.json authz-cutover verify --timeout=10m
  run_evidence_step 05-evidence.json authz-cutover evidence --timeout=10m
  run_evidence_step 06-retire-legacy.json authz-cutover retire-legacy \
    --confirm=RETIRE_LEGACY_AUTHZ_SCHEMA --timeout=10m --lock-timeout=30s

  final_schema_state="$(clone_mysql "$clone_database" -e "
    SELECT COALESCE(MAX(version), 0), COALESCE(MAX(dirty + 0), 0), COUNT(*)
    FROM schema_migrations;
  ")"
  final_legacy_table_count="$(clone_mysql "$clone_database" -e "
    SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name IN ('casbin_rule', 'authz_cutover_state');
  ")"
  final_scope_column_count="$(clone_mysql "$clone_database" -e "
    SELECT COUNT(*) FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'authz_resources'
      AND column_name = 'scope_kinds';
  ")"
  if [ "$final_schema_state" != $'27\t0\t1' ]; then
    echo "full clone rehearsal did not finish at clean schema version 27 (actual=${final_schema_state})" >&2
    exit 1
  fi
  if [ "$final_legacy_table_count" != "0" ] || [ "$final_scope_column_count" != "0" ]; then
    echo "full clone rehearsal left legacy AuthZ schema behind" >&2
    exit 1
  fi

  cat >"${evidence_dir}/07-final-schema.txt" <<EOF
iam_sha=${IAM_AUTHZ_RELEASE_SHA}
source_backup=${backup_name}
clone_container=${clone_container}
clone_database=${clone_database}
schema_version=27
schema_dirty=false
casbin_rule_absent=true
authz_cutover_state_absent=true
scope_kinds_absent=true
EOF
  chmod 0600 "${evidence_dir}/07-final-schema.txt"
  (cd "$evidence_dir" && shasum -a 256 ./* >checksums.sha256)
  chmod 0600 "${evidence_dir}/checksums.sha256"
  (cd "$evidence_dir" && shasum -a 256 -c checksums.sha256)
fi

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo '### IAM AuthZ backup clone preflight'
    echo
    echo "- IAM SHA: ${IAM_AUTHZ_RELEASE_SHA}"
    echo "- Source backup: ${backup_name} (read-only copy from serverB)"
    echo "- Clone location: Mac mini / ${clone_container} / ${clone_database}"
    echo '- Clone schema: 26 clean'
    echo "- Preflight exit code: ${preflight_status}"
    echo "- Validation mode: ${IAM_AUTHZ_CLONE_VALIDATION_MODE}"
    if [ "$IAM_AUTHZ_CLONE_VALIDATION_MODE" = "full_rehearsal" ]; then
      echo '- Final clone schema: 27 clean'
      echo '- Legacy absence: casbin_rule=true, authz_cutover_state=true, scope_kinds=true'
    else
      echo '- No apply or legacy-retirement operation was executed.'
    fi
  } >>"$GITHUB_STEP_SUMMARY"
fi

if [ "$preflight_status" -ne 0 ]; then
  echo "AuthZ backup clone preflight reproduced a failure; the persistent Mac mini clone is available for diagnosis" >&2
  exit "$preflight_status"
fi
if [ "$IAM_AUTHZ_CLONE_VALIDATION_MODE" = "full_rehearsal" ]; then
  echo "IAM AuthZ backup clone full rehearsal passed; persistent schema-27 clone remains on the Mac mini"
else
  echo "IAM AuthZ backup clone preflight passed; persistent clone remains on the Mac mini"
fi
