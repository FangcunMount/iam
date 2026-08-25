#!/usr/bin/env bash
set -Eeuo pipefail

OPERATION="${IAM_AUTHZ_CONSUMER_OPERATION:-}"
RELEASE_SHA="${IAM_AUTHZ_RELEASE_SHA:-}"
STATE_ROOT="${IAM_AUTHZ_CONSUMER_STATE_ROOT:-/opt/backups/iam/authz-cutover/runtime}"
STATE_FILE="${STATE_ROOT}/${RELEASE_SHA}.iam-containers"

case "$RELEASE_SHA" in
  ""|*[!0-9a-f]*)
    echo "IAM_AUTHZ_RELEASE_SHA must be a lowercase hexadecimal Git SHA" >&2
    exit 1
    ;;
esac
if [ "${#RELEASE_SHA}" -ne 40 ]; then
  echo "IAM_AUTHZ_RELEASE_SHA must contain 40 characters" >&2
  exit 1
fi
case "$OPERATION" in
  stop|start|status) ;;
  *) echo "unsupported IAM AuthZ consumer operation" >&2; exit 1 ;;
esac

if sudo -n true 2>/dev/null; then
  run_privileged() { sudo "$@"; }
else
  : "${SUDO_PASSWORD:?SUDO_PASSWORD is required when passwordless sudo is unavailable}"
  run_privileged() { printf '%s\n' "$SUDO_PASSWORD" | sudo -S "$@"; }
fi

running_ids() {
  run_privileged docker ps -q --filter 'name=^/iam-apiserver$'
}

prepare_state_root() {
  if [ ! -d "$STATE_ROOT" ]; then
    run_privileged mkdir -p -- "$STATE_ROOT"
  fi
  if [ -L "$STATE_ROOT" ]; then
    echo "cutover runtime state root must not be a symbolic link" >&2
    exit 1
  fi
  run_privileged chown "$(id -u):$(id -g)" "$STATE_ROOT"
  chmod 0750 "$STATE_ROOT"
}

case "$OPERATION" in
  stop)
    prepare_state_root
    temporary_state="$(mktemp)"
    trap 'rm -f "$temporary_state"' EXIT
    running_ids >"$temporary_state"
    cp -- "$temporary_state" "$STATE_FILE"
    chmod 0640 "$STATE_FILE"
    while IFS= read -r container_id; do
      [ -n "$container_id" ] || continue
      run_privileged docker stop "$container_id"
    done <"$temporary_state"
    if [ -n "$(running_ids)" ]; then
      echo "iam-apiserver is still running after the maintenance stop" >&2
      exit 1
    fi
    echo "IAM AuthZ consumers stopped: state=${STATE_FILE}"
    ;;
  start)
    if ! run_privileged test -f "$STATE_FILE"; then
      echo "cutover runtime state is missing: ${STATE_FILE}" >&2
      exit 1
    fi
    while IFS= read -r container_id; do
      [ -n "$container_id" ] || continue
      run_privileged docker start "$container_id"
    done < <(run_privileged cat "$STATE_FILE")
    echo "IAM AuthZ consumers restored from state=${STATE_FILE}"
    ;;
  status)
    count="$(running_ids | awk 'NF { count++ } END { print count + 0 }')"
    echo "IAM AuthZ consumer status: running=${count} release_sha=${RELEASE_SHA}"
    ;;
esac
