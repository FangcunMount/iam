#!/usr/bin/env bash
set -Eeuo pipefail

: "${RUNNER_SSH_KEY:?RUNNER_SSH_KEY is required}"
: "${RUNNER_SSH_HOST:?RUNNER_SSH_HOST is required}"
: "${RUNNER_SSH_USER:?RUNNER_SSH_USER is required}"

RUNNER_SSH_PORT="${RUNNER_SSH_PORT:-22}"
RUNNER_SSH_ALIAS="${RUNNER_SSH_ALIAS:-deploy-target}"
RUNNER_SSH_FALLBACK_HOST="${RUNNER_SSH_FALLBACK_HOST:-}"

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

write_config() {
  local target_host=$1
  # 每次重写隔离 config，避免旧公网或 Tailscale 地址残留。
  cat >"$CONFIG" <<EOF
Host ${RUNNER_SSH_ALIAS}
  HostName ${target_host}
  User ${RUNNER_SSH_USER}
  Port ${RUNNER_SSH_PORT}
  IdentityFile ${KEY_FILE}
  IdentitiesOnly yes
  BatchMode yes
  StrictHostKeyChecking accept-new
EOF
  chmod 600 "$CONFIG"
}

SSH=(ssh -F "${CONFIG}")

verify_target() {
  local target_host=$1 resolved_host remote_hostname
  write_config "$target_host"
  resolved_host="$("${SSH[@]}" -G "${RUNNER_SSH_ALIAS}" 2>/dev/null | awk '$1 == "hostname" { print $2; exit }')"
  echo "SSH config ready for ${RUNNER_SSH_ALIAS} (${RUNNER_SSH_USER}@${target_host}:${RUNNER_SSH_PORT})"
  echo "  IdentityFile=${KEY_FILE}"
  echo "  Config=${CONFIG}"
  echo "SSH effective HostName: ${resolved_host:-<unknown>}"

  if [ "${resolved_host:-}" != "$target_host" ]; then
    echo "SSH config resolves ${RUNNER_SSH_ALIAS} to ${resolved_host:-<empty>}, expected ${target_host}" >&2
    return 1
  fi

  if [ -n "${DEPLOY_NODE_HOSTNAME:-}" ]; then
    remote_hostname="$("${SSH[@]}" -o ConnectTimeout=20 "${RUNNER_SSH_ALIAS}" 'hostname -s 2>/dev/null || hostname' 2>/dev/null || true)"
    echo "SSH remote hostname: ${remote_hostname:-<unreachable>}"
    if [ -z "$remote_hostname" ]; then
      echo "Cannot SSH to ${target_host} for hostname precheck" >&2
      return 1
    fi
    if [ "$remote_hostname" != "$DEPLOY_NODE_HOSTNAME" ]; then
      echo "SSH lands on hostname ${remote_hostname}, expected ${DEPLOY_NODE_HOSTNAME}" >&2
      echo "Configured HostName is ${target_host}" >&2
      return 1
    fi
  fi
  return 0
}

selected_host=$RUNNER_SSH_HOST
if ! verify_target "$selected_host"; then
  if [ -z "$RUNNER_SSH_FALLBACK_HOST" ] || [ "$RUNNER_SSH_FALLBACK_HOST" = "$selected_host" ]; then
    echo "FATAL: cannot verify SSH deployment target ${selected_host}" >&2
    exit 1
  fi
  echo "Primary SSH host ${selected_host} unavailable; falling back to ${RUNNER_SSH_FALLBACK_HOST}." >&2
  selected_host=$RUNNER_SSH_FALLBACK_HOST
  if ! verify_target "$selected_host"; then
    echo "FATAL: neither primary nor fallback SSH target passed hostname verification" >&2
    exit 1
  fi
fi

RUNNER_SSH_HOST=$selected_host
if [ -n "${GITHUB_ENV:-}" ]; then
  {
    echo "RUNNER_SSH_KEY_FILE=${KEY_FILE}"
    echo "RUNNER_SSH_CONFIG=${CONFIG}"
    echo "RUNNER_SSH_HOST=${RUNNER_SSH_HOST}"
  } >>"${GITHUB_ENV}"
fi
echo "Selected SSH deployment host: ${RUNNER_SSH_HOST}"
