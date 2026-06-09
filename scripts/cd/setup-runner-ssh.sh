#!/usr/bin/env bash
set -Eeuo pipefail

: "${RUNNER_SSH_KEY:?RUNNER_SSH_KEY is required}"
: "${RUNNER_SSH_HOST:?RUNNER_SSH_HOST is required}"
: "${RUNNER_SSH_USER:?RUNNER_SSH_USER is required}"

RUNNER_SSH_PORT="${RUNNER_SSH_PORT:-22}"
RUNNER_SSH_ALIAS="${RUNNER_SSH_ALIAS:-deploy-target}"
KEY_FILE="${RUNNER_SSH_KEY_FILE:-$HOME/.ssh/runner_${RUNNER_SSH_ALIAS}_key}"
CONFIG="${RUNNER_SSH_CONFIG:-$HOME/.ssh/config}"
BEGIN_MARKER="# BEGIN iam-cd ${RUNNER_SSH_ALIAS}"
END_MARKER="# END iam-cd ${RUNNER_SSH_ALIAS}"

mkdir -p "$(dirname "$KEY_FILE")"
chmod 700 "$(dirname "$KEY_FILE")"
printf '%s\n' "$RUNNER_SSH_KEY" >"$KEY_FILE"
chmod 600 "$KEY_FILE"

touch "$CONFIG"
chmod 600 "$CONFIG"

remove_managed_ssh_block() {
  local file="$1"
  if ! grep -qF "$BEGIN_MARKER" "$file" 2>/dev/null; then
    return 0
  fi
  awk -v begin="$BEGIN_MARKER" -v end="$END_MARKER" '
    $0 == begin { skip = 1; next }
    $0 == end { skip = 0; next }
    !skip { print }
  ' "$file" >"${file}.iam-cd.tmp"
  mv "${file}.iam-cd.tmp" "$file"
}

remove_legacy_ssh_host_block() {
  local alias="$1" file="$2"
  awk -v host="$alias" '
    $1 == "Host" && $2 == host { skip = 1; next }
    skip && $1 == "Host" { skip = 0 }
    skip { next }
    { print }
  ' "$file" >"${file}.iam-cd.tmp"
  mv "${file}.iam-cd.tmp" "$file"
}

remove_managed_ssh_block "$CONFIG"
remove_legacy_ssh_host_block "$RUNNER_SSH_ALIAS" "$CONFIG"

cat >>"$CONFIG" <<EOF

${BEGIN_MARKER}
Host ${RUNNER_SSH_ALIAS}
  HostName ${RUNNER_SSH_HOST}
  User ${RUNNER_SSH_USER}
  Port ${RUNNER_SSH_PORT}
  IdentityFile ${KEY_FILE}
  StrictHostKeyChecking accept-new
${END_MARKER}
EOF

resolved_host="$(ssh -G "${RUNNER_SSH_ALIAS}" 2>/dev/null | awk '$1 == "hostname" { print $2; exit }')"
echo "SSH config ready for ${RUNNER_SSH_ALIAS} (${RUNNER_SSH_USER}@${RUNNER_SSH_HOST}:${RUNNER_SSH_PORT})"
echo "SSH effective HostName: ${resolved_host:-<unknown>}"

if [ "${resolved_host:-}" != "$RUNNER_SSH_HOST" ]; then
  echo "FATAL: ~/.ssh/config resolves ${RUNNER_SSH_ALIAS} to ${resolved_host:-<empty>}, expected ${RUNNER_SSH_HOST}" >&2
  exit 1
fi

if [ -n "${DEPLOY_NODE_HOSTNAME:-}" ]; then
  remote_hostname="$(ssh -o BatchMode=yes -o ConnectTimeout=20 "${RUNNER_SSH_ALIAS}" 'hostname -s 2>/dev/null || hostname' 2>/dev/null || true)"
  echo "SSH remote hostname: ${remote_hostname:-<unreachable>}"
  if [ -z "$remote_hostname" ]; then
    echo "FATAL: cannot SSH to ${RUNNER_SSH_ALIAS} for hostname precheck" >&2
    exit 1
  fi
  if [ "$remote_hostname" != "$DEPLOY_NODE_HOSTNAME" ]; then
    echo "FATAL: SSH lands on hostname ${remote_hostname}, expected ${DEPLOY_NODE_HOSTNAME}" >&2
    echo "Configured HostName is ${RUNNER_SSH_HOST}; check runner ~/.ssh/config for stale deploy-target entries." >&2
    exit 1
  fi
fi
