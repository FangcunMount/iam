#!/usr/bin/env bash
set -euo pipefail

repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
sdk=${1:?Usage: reliable-messaging-fallback-proof.sh /absolute/path/to/reliable-messaging-v0.2.1}
[[ "$sdk" = /* && -f "$sdk/tests/integration/compose.yaml" ]] || {
  echo 'SDK v0.2.1 checkout with the isolated Compose fixture is required' >&2
  exit 1
}
project="rm-iam-fallback-${GITHUB_RUN_ID:-local}-$$"
compose=(docker compose --project-name "$project" --file "$sdk/tests/integration/compose.yaml")
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/$project.XXXXXX")
cleanup() {
  result=$?
  trap - EXIT
  if ((result != 0)); then
    "${compose[@]}" logs --tail 30 --no-color || true
  fi
  if ! "${compose[@]}" down --volumes --remove-orphans --timeout 10; then
    result=1
  fi
  rm -rf -- "$build_dir"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cd "$repo"
GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags=reliable_messaging \
  -o "$build_dir/iam-fallback-proof" ./internal/apiserver/infra/authz/integration
GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags=reliable_messaging \
  -o "$build_dir/iam-fallback-startup-proof" ./internal/apiserver/process
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqd
"${compose[@]}" cp "$build_dir/iam-fallback-proof" mysql:/tmp/iam-fallback-proof
"${compose[@]}" cp "$build_dir/iam-fallback-startup-proof" mysql:/tmp/iam-fallback-startup-proof
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000006_add_domain_event_outbox.up.sql" mysql:/tmp/iam-old-outbox.sql
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000038_standard_message_outbox.up.sql" mysql:/tmp/iam-rm-outbox.sql
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000039_reliable_messaging_failure_state.up.sql" mysql:/tmp/iam-rm-failure-state.sql
"${compose[@]}" cp "$repo/internal/pkg/migration/migrations/000040_iam_nsq_failure_audit.up.sql" mysql:/tmp/iam-rm-failure-audit.sql
"${compose[@]}" cp "$repo/configs/events.yaml" mysql:/tmp/iam-events.yaml
"${compose[@]}" cp "$repo/configs/grpc_assignment_constraints.yaml" mysql:/tmp/iam-assignment-constraints.yaml
"${compose[@]}" exec -T \
  -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml \
  -e RM_IAM_OUTBOX_SCHEMA=/tmp/iam-old-outbox.sql \
  -e RM_IAM_OUTBOX_UPGRADE=/tmp/iam-rm-outbox.sql \
  -e RM_IAM_OUTBOX_FAILURE_UPGRADE=/tmp/iam-rm-failure-state.sql \
  -e RM_IAM_FAILURE_AUDIT_UPGRADE=/tmp/iam-rm-failure-audit.sql \
  -e RM_IAM_NSQ_TCP=nsqd:4150 \
  -e 'IAM_AUTHZ_TEST_MYSQL_DSN=root@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC' \
  mysql /tmp/iam-fallback-proof -test.run '^TestReliableMessagingPlatformWiring$' -test.v
"${compose[@]}" exec -T \
  -e RM_IAM_PROCESS_MYSQL=1 \
  -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml \
  -e RM_IAM_ASSIGNMENT_CONSTRAINTS=/tmp/iam-assignment-constraints.yaml \
  -e RM_IAM_NSQ_TCP=nsqd:4150 \
  mysql /tmp/iam-fallback-startup-proof -test.run '^TestFallbackSchema40APICompositionReadiness$' -test.v
