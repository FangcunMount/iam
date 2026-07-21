#!/usr/bin/env bash
set -Eeuo pipefail

: "${RUNNER_SSH_KEY:?RUNNER_SSH_KEY is required}"
: "${RUNNER_SSH_HOST:?RUNNER_SSH_HOST is required}"
: "${RUNNER_SSH_USER:?RUNNER_SSH_USER is required}"

RUNNER_SSH_PORT="${RUNNER_SSH_PORT:-22}"
RUNNER_SSH_ALIAS="${RUNNER_SSH_ALIAS:-deploy-target}"

# 写到 RUNNER_TEMP，避免覆盖 runner 用户 ~/.ssh 下的个人密钥/config
SSH_HOME="${RUNNER_TEMP:-/tmp}/iam-ssh-${GITHUB_RUN_ID:-$$}"
mkdir -p "${SSH_HOME}"
chmod 700 "${SSH_HOME}"

KEY_FILE="${RUNNER_SSH_KEY_FILE:-${SSH_HOME}/runner_${RUNNER_SSH_ALIAS}_key}"
CONFIG="${RUNNER_SSH_CONFIG:-${SSH_HOME}/config}"

umask 077
printf '%s\n' "$RUNNER_SSH_KEY" | tr -d '\r' >"$KEY_FILE"
chmod 600 "$KEY_FILE"
ssh-keygen -lf "$KEY_FILE"

# 每次重写隔离 config，避免旧 Tailscale IP 残留导致仍走 DERP
cat >"$CONFIG" <<EOF
Host ${RUNNER_SSH_ALIAS}
  HostName ${RUNNER_SSH_HOST}
  User ${RUNNER_SSH_USER}
  Port ${RUNNER_SSH_PORT}
  IdentityFile ${KEY_FILE}
  IdentitiesOnly yes
  BatchMode yes
  StrictHostKeyChecking accept-new
EOF
chmod 600 "$CONFIG"

if [ -n "${GITHUB_ENV:-}" ]; then
  {
    echo "RUNNER_SSH_KEY_FILE=${KEY_FILE}"
    echo "RUNNER_SSH_CONFIG=${CONFIG}"
  } >>"${GITHUB_ENV}"
fi

SSH=(ssh -F "${CONFIG}")

resolved_host="$("${SSH[@]}" -G "${RUNNER_SSH_ALIAS}" 2>/dev/null | awk '$1 == "hostname" { print $2; exit }')"
echo "SSH config ready for ${RUNNER_SSH_ALIAS} (${RUNNER_SSH_USER}@${RUNNER_SSH_HOST}:${RUNNER_SSH_PORT})"
echo "  IdentityFile=${KEY_FILE}"
echo "  Config=${CONFIG}"
echo "SSH effective HostName: ${resolved_host:-<unknown>}"

if [ "${resolved_host:-}" != "$RUNNER_SSH_HOST" ]; then
  echo "FATAL: SSH config resolves ${RUNNER_SSH_ALIAS} to ${resolved_host:-<empty>}, expected ${RUNNER_SSH_HOST}" >&2
  exit 1
fi

if [ -n "${DEPLOY_NODE_HOSTNAME:-}" ]; then
  remote_hostname="$("${SSH[@]}" -o ConnectTimeout=20 "${RUNNER_SSH_ALIAS}" 'hostname -s 2>/dev/null || hostname' 2>/dev/null || true)"
  echo "SSH remote hostname: ${remote_hostname:-<unreachable>}"
  if [ -z "$remote_hostname" ]; then
    echo "FATAL: cannot SSH to ${RUNNER_SSH_ALIAS} for hostname precheck" >&2
    exit 1
  fi
  if [ "$remote_hostname" != "$DEPLOY_NODE_HOSTNAME" ]; then
    echo "FATAL: SSH lands on hostname ${remote_hostname}, expected ${DEPLOY_NODE_HOSTNAME}" >&2
    echo "Configured HostName is ${RUNNER_SSH_HOST}" >&2
    exit 1
  fi
fi
