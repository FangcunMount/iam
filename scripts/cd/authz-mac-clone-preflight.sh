#!/usr/bin/env bash
set -Eeuo pipefail

: "${IAM_AUTHZ_CLONE_CONFIRMATION:?IAM_AUTHZ_CLONE_CONFIRMATION is required}"
: "${IAM_AUTHZ_RELEASE_SHA:?IAM_AUTHZ_RELEASE_SHA is required}"
: "${IAM_AUTHZ_CLONE_BACKUP_FILE:?IAM_AUTHZ_CLONE_BACKUP_FILE is required}"
: "${IAM_AUTHZ_CLONE_MYSQL_PASSWORD:?IAM_AUTHZ_CLONE_MYSQL_PASSWORD is required}"
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"

if [ "$IAM_AUTHZ_CLONE_CONFIRMATION" != "PREFLIGHT_AUTHZ_BACKUP_CLONE" ]; then
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
docker info >/dev/null

readonly clone_image="mysql:8.0"
readonly clone_container="iam-authz-preflight-mysql"
readonly clone_data_volume="iam-authz-preflight-mysql-data"
readonly clone_secret_volume="iam-authz-preflight-mysql-secrets"
readonly clone_database="iam_authz_preflight"
readonly clone_port="33306"

docker pull "$clone_image" >/dev/null
docker volume inspect "$clone_data_volume" >/dev/null 2>&1 || docker volume create "$clone_data_volume" >/dev/null
docker volume inspect "$clone_secret_volume" >/dev/null 2>&1 || docker volume create "$clone_secret_volume" >/dev/null

# Store the clone-only credential in a dedicated Docker volume. It therefore
# survives runner cleanup without appearing in the long-lived container env.
printf '%s' "$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" |
  docker run --rm -i \
    --volume "${clone_secret_volume}:/secrets" \
    --entrypoint sh "$clone_image" \
    -c 'umask 077; cat > /secrets/root-password'

if docker container inspect "$clone_container" >/dev/null 2>&1; then
  docker rm -f "$clone_container" >/dev/null
fi
docker run -d \
  --name "$clone_container" \
  --restart unless-stopped \
  --publish "127.0.0.1:${clone_port}:3306" \
  --cpus "1.0" \
  --memory "2g" \
  --memory-swap "2g" \
  --pids-limit "512" \
  --env MYSQL_ROOT_PASSWORD_FILE=/run/secrets/mysql-root-password \
  --env MYSQL_ROOT_HOST=% \
  --volume "${clone_data_volume}:/var/lib/mysql" \
  --volume "${clone_secret_volume}:/run/secrets:ro" \
  "$clone_image" >/dev/null

clone_mysql() {
  docker exec -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
    mysql --protocol=socket -uroot --batch --skip-column-names "$@"
}

ready=0
for _ in $(seq 1 30); do
  if docker exec -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
    mysqladmin --protocol=socket -uroot ping --silent >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [ "$ready" != "1" ]; then
  docker logs --tail 100 "$clone_container" >&2 || true
  echo "Mac mini clone MySQL did not become ready" >&2
  exit 1
fi

# Destructive scope is intentionally non-configurable and limited to the
# dedicated clone database. The container and data volume remain for follow-up
# investigation, while each run restores a clean copy of the selected backup.
clone_mysql -e 'DROP DATABASE IF EXISTS `iam_authz_preflight`; CREATE DATABASE `iam_authz_preflight` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
gzip -dc "$IAM_AUTHZ_CLONE_BACKUP_FILE" |
  docker exec -i -e MYSQL_PWD="$IAM_AUTHZ_CLONE_MYSQL_PASSWORD" "$clone_container" \
    mysql -uroot --default-character-set=utf8mb4 "$clone_database"

schema_state="$(clone_mysql "$clone_database" -e "
  SELECT CONCAT(COALESCE(MAX(version), 0), '\t', COALESCE(MAX(dirty + 0), 0), '\t', COUNT(*))
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
  SELECT CONCAT(COALESCE(MAX(version), 0), '\t', COALESCE(MAX(dirty + 0), 0), '\t', COUNT(*))
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

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo '### IAM AuthZ backup clone preflight'
    echo
    echo "- IAM SHA: ${IAM_AUTHZ_RELEASE_SHA}"
    echo "- Source backup: ${backup_name} (read-only copy from serverB)"
    echo "- Clone location: Mac mini / ${clone_container} / ${clone_database}"
    echo '- Clone schema: 26 clean'
    echo "- Preflight exit code: ${preflight_status}"
    echo '- No apply or legacy-retirement operation was executed.'
  } >>"$GITHUB_STEP_SUMMARY"
fi

if [ "$preflight_status" -ne 0 ]; then
  echo "AuthZ backup clone preflight reproduced a failure; the persistent Mac mini clone is available for diagnosis" >&2
  exit "$preflight_status"
fi
echo "IAM AuthZ backup clone preflight passed; persistent clone remains on the Mac mini"
