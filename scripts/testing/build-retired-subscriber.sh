#!/usr/bin/env bash
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
source_commit=a929f5301ed8265a8f44726b474da27ae10cb1c1
output=${1:?absolute output file required}
[[ "$output" = /* && ! -e "$output" ]] || exit 1
[[ $(git -C "$repo" rev-parse "$source_commit^{commit}") == "$source_commit" ]] || exit 1
snapshot=$(mktemp -d "${TMPDIR:-/tmp}/iam-retired-subscriber.XXXXXX")
trap 'rm -rf -- "$snapshot"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
git -C "$repo" archive "$source_commit" | tar -x -C "$snapshot"
(cd "$snapshot" && GOWORK=off go test -c -tags=reliable_messaging -o "$output" ./internal/apiserver/container/authz)
echo "Historical subscriber binary source: $source_commit"
