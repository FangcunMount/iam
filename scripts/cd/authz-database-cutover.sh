#!/usr/bin/env bash
set -Eeuo pipefail

: "${IAM_AUTHZ_RELEASE_SHA:?IAM_AUTHZ_RELEASE_SHA is required}"
: "${IAM_AUTHZ_QS_RELEASE_SHA:?IAM_AUTHZ_QS_RELEASE_SHA is required}"
: "${IAM_AUTHZ_QS_STOP_RUN_ID:?IAM_AUTHZ_QS_STOP_RUN_ID is required}"
: "${IAM_AUTHZ_CUTOVER_CONFIRMATION:?IAM_AUTHZ_CUTOVER_CONFIRMATION is required}"
: "${IAM_AUTHZ_MAINTENANCE_BINARY:?IAM_AUTHZ_MAINTENANCE_BINARY is required}"

if [ "$IAM_AUTHZ_CUTOVER_CONFIRMATION" != "CUTOVER_AUTHZ_V3" ]; then
  echo "invalid production AuthZ cutover confirmation" >&2
  exit 1
fi
for sha in "$IAM_AUTHZ_RELEASE_SHA" "$IAM_AUTHZ_QS_RELEASE_SHA"; do
  if [ "${#sha}" -ne 40 ] || [ -n "${sha//[0-9a-f]/}" ]; then
    echo "release SHA evidence must contain exactly 40 lowercase hexadecimal characters" >&2
    exit 1
  fi
done
if ! [[ "$IAM_AUTHZ_QS_STOP_RUN_ID" =~ ^[1-9][0-9]*$ ]]; then
  echo "qs-server stop workflow run ID is invalid" >&2
  exit 1
fi
expected_binary="/tmp/iam-authz-cutover-${IAM_AUTHZ_RELEASE_SHA}/iam-maintenance"
if [ "$IAM_AUTHZ_MAINTENANCE_BINARY" != "$expected_binary" ] || [ ! -f "$IAM_AUTHZ_MAINTENANCE_BINARY" ]; then
  echo "IAM maintenance binary is missing or not executable" >&2
  exit 1
fi
chmod 0500 "$IAM_AUTHZ_MAINTENANCE_BINARY"

EVIDENCE_ROOT="${IAM_AUTHZ_EVIDENCE_ROOT:-/opt/backups/iam/authz-cutover}"
EVIDENCE_DIR="${EVIDENCE_ROOT}/${IAM_AUTHZ_RELEASE_SHA}"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

if sudo -n true 2>/dev/null; then
  run_privileged() { sudo "$@"; }
else
  : "${SUDO_PASSWORD:?SUDO_PASSWORD is required when passwordless sudo is unavailable}"
  run_privileged() { printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"; }
fi

if [ -n "$(run_privileged docker ps -q --filter 'name=^/iam-apiserver$')" ]; then
  echo "iam-apiserver must be stopped before the database cutover" >&2
  exit 1
fi

run_step() {
  local name="$1"
  shift
  echo "Running AuthZ cutover step: ${name}"
  "$IAM_AUTHZ_MAINTENANCE_BINARY" "$@" | tee "$TEMP_DIR/${name}.json"
}

run_step 01-migrate-additive authz-cutover migrate-additive \
  --confirm=MIGRATE_AUTHZ_ADDITIVE_SCHEMA --timeout=10m --lock-timeout=30s
run_step 02-preflight authz-cutover preflight --timeout=10m
run_step 03-apply authz-cutover apply \
  --confirm=APPLY_AUTHZ_CUTOVER --timeout=10m --lock-timeout=30s
run_step 04-verify authz-cutover verify --timeout=10m
run_step 05-evidence authz-cutover evidence --timeout=10m

cat >"$TEMP_DIR/release-evidence.txt" <<EOF
iam_sha=${IAM_AUTHZ_RELEASE_SHA}
qs_sha=${IAM_AUTHZ_QS_RELEASE_SHA}
qs_stop_run_id=${IAM_AUTHZ_QS_STOP_RUN_ID}
cutover_completed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
(cd "$TEMP_DIR" && sha256sum ./*.json >checksums.sha256)

run_privileged install -d -m 0750 "$EVIDENCE_DIR"
for evidence_file in "$TEMP_DIR"/*.json "$TEMP_DIR"/checksums.sha256 "$TEMP_DIR"/release-evidence.txt; do
  run_privileged install -m 0640 "$evidence_file" "$EVIDENCE_DIR/$(basename "$evidence_file")"
done

# From this point a failure requires restoring the maintenance-window backup;
# legacy services must not be restarted against a partially retired schema.
run_step 06-retire-legacy authz-cutover retire-legacy \
  --confirm=RETIRE_LEGACY_AUTHZ_SCHEMA --timeout=10m --lock-timeout=30s
run_privileged install -m 0640 "$TEMP_DIR/06-retire-legacy.json" "$EVIDENCE_DIR/06-retire-legacy.json"
(cd "$TEMP_DIR" && sha256sum ./*.json >checksums.sha256)
run_privileged install -m 0640 "$TEMP_DIR/checksums.sha256" "$EVIDENCE_DIR/checksums.sha256"
run_privileged sh -c 'cd "$1" && sha256sum -c checksums.sha256' sh "$EVIDENCE_DIR"
echo "IAM AuthZ database cutover completed: evidence=${EVIDENCE_DIR}"
