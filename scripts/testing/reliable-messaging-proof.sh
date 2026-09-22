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
# Temporary workspace: neither repository's go.mod is rewritten.
(cd "$build_dir" && GOWORK=off go work init "$repo" "$sdk")
# Host lifecycle contract uses actual scheduling/shutdown functions with controlled
# dispatch/close boundaries. It does not substitute for the database/broker proofs.
(cd "$repo" && GOWORK="$build_dir/go.work" go test -race -tags=reliable_messaging ./internal/apiserver/process -run '^TestReliableMessagingShutdownJoinBoundary$' -count=10)
(cd "$repo" && GOWORK="$build_dir/go.work" CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags=reliable_messaging -o "$build_dir/proof" ./internal/apiserver/infra/authz/integration)
(cd "$repo" && GOWORK="$build_dir/go.work" CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -o "$build_dir/migration-proof" ./internal/pkg/migration)
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqd
"${compose[@]}" cp "$build_dir/proof" mysql:/tmp/iam-proof
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000006_add_domain_event_outbox.up.sql" mysql:/tmp/iam-old-outbox.sql
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000038_reliable_messaging_claims.up.sql" mysql:/tmp/iam-rm-upgrade.sql
"${compose[@]}" exec -T -e RM_IAM_OUTBOX_UPGRADE='/tmp/iam-rm-upgrade.sql' -e RM_IAM_OUTBOX_SCHEMA='/tmp/iam-old-outbox.sql' -e RM_IAM_NSQ_TCP='nsqd:4150' -e RM_IAM_NSQ_HTTP='http://nsqd:4151' -e IAM_AUTHZ_TEST_MYSQL_DSN='root@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC' mysql /tmp/iam-proof -test.run '^TestReliableMessaging(OriginalUoW|PolicyConsumer|HistoricalFencing|HistoricalStore|HistoricalStager)$' -test.v

# Real production migration files and full fresh bootstrap use separate,
# disposable databases in this same isolated MySQL container.
"${compose[@]}" cp "$build_dir/migration-proof" mysql:/tmp/iam-migration-proof
"${compose[@]}" cp "$repo/configs/mysql/bootstrap.sql" mysql:/tmp/iam-bootstrap.sql
"${compose[@]}" cp "$repo/configs/events.yaml" mysql:/tmp/iam-events.yaml
"${compose[@]}" exec -T mysql mysql -uroot -e 'CREATE DATABASE rm_iam_migration; CREATE DATABASE rm_iam_full_chain;'
"${compose[@]}" exec -T -e IAM_RM_MIGRATION_REQUIRED=1 -e MYSQL_HOST=127.0.0.1 -e MYSQL_USER=root -e MYSQL_PASSWORD='' -e MYSQL_DATABASE=rm_iam_migration mysql /tmp/iam-migration-proof -test.run '^TestReliableMessagingSchemaUpgradeAndRollbackMySQL$' -test.v
"${compose[@]}" exec -T -e MYSQL_HOST=127.0.0.1 -e MYSQL_USER=root -e MYSQL_PASSWORD='' -e MYSQL_DATABASE=rm_iam_full_chain -e RM_IAM_BOOTSTRAP_SQL=/tmp/iam-bootstrap.sql -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml mysql /tmp/iam-migration-proof -test.run '^TestFullMigrationChainAndBootstrapMySQL$' -test.v
