#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
sdk=${1:?Usage: reliable-messaging-business-proof.sh /absolute/sdk /absolute/qs-proof-checkout}
qs=${2:?QS checkout with the tagged business proof required}
[[ "$sdk" = /* && -f "$sdk/tests/integration/compose.yaml" && "$qs" = /* && -f "$qs/internal/pkg/iamauth/reliable_messaging_roundtrip_test.go" ]] || exit 1
context=$(docker context show)
endpoint=$(docker context inspect "$context" --format '{{.Endpoints.docker.Host}}')
[[ -z ${DOCKER_HOST:-} && "$endpoint" = unix://* ]] || { echo 'Local Unix Docker context required' >&2; exit 1; }
project="rm-business-$(date +%s)-$$-$RANDOM"
compose=(docker compose --project-name "$project" --file "$sdk/tests/integration/compose.yaml" --file "$repo/scripts/testing/reliable-messaging-business-compose.yaml")
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/$project.XXXXXX")
cleanup(){
 result=$?
 trap - EXIT
 if ((result!=0));then "${compose[@]}" logs --tail 15 --no-color || true;fi
 if ! "${compose[@]}" down --volumes --remove-orphans --timeout 10;then result=1;fi
 rm -rf -- "$build_dir"
 exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
architecture=$(docker info --format '{{.Architecture}}')
case "$architecture" in aarch64|arm64) goarch=arm64;;x86_64|amd64) goarch=amd64;;*) exit 1;;esac
# Two separately built modules retain their declared dependencies and wire ABI.
(cd "$repo" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags=reliable_messaging -o "$build_dir/iam-business" ./internal/apiserver/container/authz)
(cd "$qs" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags=reliable_messaging -o "$build_dir/qs-business" ./internal/pkg/iamauth)
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqlookupd nsqd
"${compose[@]}" cp "$build_dir/iam-business" mysql:/tmp/iam-business
"${compose[@]}" cp "$build_dir/qs-business" mysql:/tmp/qs-business
"${compose[@]}" cp "$repo/configs/events.yaml" mysql:/tmp/iam-events.yaml
"${compose[@]}" cp "$repo/configs/grpc_acl.yaml" mysql:/tmp/iam-grpc-acl.yaml
"${compose[@]}" exec -T -e RM_IAM_GRPC_ACL=/tmp/iam-grpc-acl.yaml -e RM_BUSINESS_REQUIRED=1 -e RM_QS_PROOF=/tmp/qs-business -e RM_IAM_EVENTS_CATALOG=/tmp/iam-events.yaml -e RM_IAM_NSQ_TCP=nsqd:4150 -e RM_IAM_NSQ_HTTP=http://nsqd:4151 -e RM_NSQ_LOOKUP=nsqlookupd:4161 -e IAM_AUTHZ_TEST_MYSQL_DSN='root@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC' mysql /tmp/iam-business -test.run '^TestReliableMessagingIAMQSBusinessRoundtrip$' -test.v
