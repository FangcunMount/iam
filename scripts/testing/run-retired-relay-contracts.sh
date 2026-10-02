#!/usr/bin/env bash
# Retired IAM Relay contracts execute their original immutable source.
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
source_commit=a929f5301ed8265a8f44726b474da27ae10cb1c1
[[ $(git -C "$repo" rev-parse "$source_commit^{commit}") == "$source_commit" ]] || exit 1
kind=${1:-unit}
case "$kind" in unit|lifecycle) ;; *) echo 'Usage: run-retired-relay-contracts.sh unit|lifecycle' >&2; exit 2;; esac
snapshot=$(mktemp -d "${TMPDIR:-/tmp}/iam-retired-relay-test.XXXXXX")
trap 'rm -rf -- "$snapshot"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
git -C "$repo" archive "$source_commit" | tar -x -C "$snapshot"
echo "Historical source: $source_commit; case: $kind"
cd "$snapshot"
export GOWORK=off
case "$kind" in
 unit) go test -count=1 -run '^TestOutboxRelay' ./internal/apiserver/infra/messaging ;;
 lifecycle) go test -tags=reliable_messaging -count=1 -run '^TestReliableMessagingShutdownJoinBoundary$' ./internal/apiserver/process ;;
esac
