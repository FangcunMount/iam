#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
sdk=${1:?Usage: reliable-messaging-proof.sh /absolute/path/to/reliable-messaging}
[[ "$sdk" = /* && -f "$sdk/tests/integration/compose.yaml" ]] || { echo 'SDK absolute checkout required' >&2; exit 1; }
context=$(docker context show)
endpoint=$(docker context inspect "$context" --format '{{.Endpoints.docker.Host}}')
[[ -z ${DOCKER_HOST:-} && "$endpoint" = unix://* ]] || { echo 'Local Unix Docker context required' >&2; exit 1; }
project="rm-iam-proof-$(date +%s)-$$-$RANDOM"
compose=(docker compose --project-name "$project" --file "$sdk/tests/integration/compose.yaml")
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/$project.XXXXXX")
cleanup(){
 result=$?
 trap - EXIT
 if ((result!=0));then "${compose[@]}" logs --tail 30 --no-color || true;fi
 if ! "${compose[@]}" down --volumes --remove-orphans --timeout 10;then result=1;fi
 rm -rf -- "$build_dir"
 exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
architecture=$(docker info --format '{{.Architecture}}')
case "$architecture" in aarch64|arm64) goarch=arm64;;x86_64|amd64) goarch=amd64;;*) exit 1;;esac
# Compile the declared IAM dependency; the SDK checkout supplies isolated fixtures only.
# Host lifecycle contract uses actual scheduling/shutdown functions with controlled
# dispatch/close boundaries. It does not substitute for the database/broker proofs.
(cd "$repo" && GOWORK=off go test -race -tags=reliable_messaging ./internal/apiserver/process -run '^TestReliable(MessagingShutdownJoinBoundary|ShutdownRetainsResourcesUntilRetryDrains|ShutdownFailureExitsNonzero)$' -count=10)
(cd "$repo" && GOWORK=off go test -race ./internal/apiserver/infra/messaging -run '^TestReliableRuntime' -count=10)
(cd "$repo" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags=reliable_messaging -o "$build_dir/proof" ./internal/apiserver/infra/authz/integration)
(cd "$repo" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -o "$build_dir/migration-proof" ./internal/pkg/migration)
(cd "$repo" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -o "$build_dir/iam-maintenance" ./cmd/iam-maintenance)
(cd "$repo" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags=reliable_messaging -o "$build_dir/maintenance-proof" ./cmd/iam-maintenance)
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqd
"${compose[@]}" cp "$build_dir/proof" mysql:/tmp/iam-proof
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000006_add_domain_event_outbox.up.sql" mysql:/tmp/iam-old-outbox.sql
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000038_standard_message_outbox.up.sql" mysql:/tmp/iam-rm-upgrade.sql
"${compose[@]}" cp "$repo/configs/events.yaml" mysql:/tmp/iam-events.yaml
"${compose[@]}" exec -T -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml -e RM_IAM_OUTBOX_UPGRADE='/tmp/iam-rm-upgrade.sql' -e RM_IAM_OUTBOX_SCHEMA='/tmp/iam-old-outbox.sql' -e RM_IAM_NSQ_TCP='nsqd:4150' -e RM_IAM_NSQ_HTTP='http://nsqd:4151' -e IAM_AUTHZ_TEST_MYSQL_DSN='root@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC' mysql /tmp/iam-proof -test.run '^TestReliableMessaging(OriginalUoW|PolicyConsumer|HistoricalFencing|PlatformWiring|StandardPreflight|StandardUoW|MaintenanceStager)$' -test.v

# Real production migration files and full fresh bootstrap use separate,
# disposable databases in this same isolated MySQL container.
"${compose[@]}" cp "$build_dir/migration-proof" mysql:/tmp/iam-migration-proof
"${compose[@]}" cp "$repo/configs/mysql/bootstrap.sql" mysql:/tmp/iam-bootstrap.sql
"${compose[@]}" exec -T mysql mysql -uroot -e 'CREATE DATABASE rm_iam_migration; CREATE DATABASE rm_iam_full_chain;'
"${compose[@]}" exec -T -e IAM_RM_MIGRATION_REQUIRED=1 -e MYSQL_HOST=127.0.0.1 -e MYSQL_USER=root -e MYSQL_PASSWORD='' -e MYSQL_DATABASE=rm_iam_migration mysql /tmp/iam-migration-proof -test.run '^TestReliableMessagingSchemaUpgradeAndRollbackMySQL$' -test.v
"${compose[@]}" exec -T -e MYSQL_HOST=127.0.0.1 -e MYSQL_USER=root -e MYSQL_PASSWORD='' -e MYSQL_DATABASE=rm_iam_full_chain -e RM_IAM_BOOTSTRAP_SQL=/tmp/iam-bootstrap.sql -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml mysql /tmp/iam-migration-proof -test.run '^TestFullMigrationChainAndBootstrapMySQL$' -test.v

# Exercise the actual read-only CLI against the fully migrated disposable DB.
"${compose[@]}" cp "$build_dir/iam-maintenance" mysql:/tmp/iam-maintenance
# Fresh bootstrap deliberately leaves legacy notifications for its legacy
# phase; the SDK data gate must reject them until the old chain is drained.
if report=$("${compose[@]}" exec -T -e IAM_APISERVER_MYSQL_HOST=127.0.0.1 -e IAM_APISERVER_MYSQL_USERNAME=root -e IAM_APISERVER_MYSQL_PASSWORD='' -e IAM_APISERVER_MYSQL_DATABASE=rm_iam_full_chain mysql /tmp/iam-maintenance reliable-messaging preflight --event-catalog=/tmp/iam-events.yaml --target=sdk); then
  echo 'SDK preflight incorrectly accepted unfinished bootstrap messages' >&2
  exit 1
fi
python3 -c 'import json,sys; r=json.loads(sys.argv[1]); assert r["legacy_unfinished"]==2 and r["standard_rows"]==0 and not r["sdk_data_ready"] and r["legacy_rollback_data_ready"] and not r["cutover_authorized"]; print("PASS fresh bootstrap blocks SDK cutover until legacy drain")' "$report"

"${compose[@]}" cp "$build_dir/maintenance-proof" mysql:/tmp/iam-maintenance-proof
"${compose[@]}" exec -T -e TZ=UTC -e IAM_RM_TIMEZONE_REQUIRED=1 -e IAM_APISERVER_MYSQL_HOST=127.0.0.1 -e IAM_APISERVER_MYSQL_USERNAME=root -e IAM_APISERVER_MYSQL_PASSWORD='' -e IAM_APISERVER_MYSQL_DATABASE=rm_iam_full_chain -e MYSQL_HOST=127.0.0.1 -e MYSQL_USER=root -e MYSQL_PASSWORD='' -e MYSQL_DATABASE=rm_iam_full_chain mysql /tmp/iam-maintenance-proof -test.run '^TestMaintenanceDatabaseTimezoneMySQL$' -test.v
