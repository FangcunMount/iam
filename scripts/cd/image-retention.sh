#!/usr/bin/env bash
# Shared host lock: acquire BEFORE loading/pulling or replacing any image/container.
# SUDO is the existing deploy wrapper (sudo or sudo_pw).
acquire_image_deploy_lock() {
  command -v python3 >/dev/null
  command -v flock >/dev/null
  local lock=/var/lib/fangcun-image-retention/deploy.lock identity
  # Host provisioning owns this inode and its permissions. A deployment only
  # borrows it; missing or changed locks require an administrator's diagnosis.
  if [ -L "$lock" ] || [ ! -f "$lock" ] || \
      [ "$(readlink -f "$lock")" != "$lock" ] || \
      [ "$(stat -c '%u:%a' "$(dirname "$lock")")" != '0:755' ] || \
      [ "$(stat -c '%u:%a' "$lock")" != '0:666' ]; then
    echo 'Original shared deployment lock unavailable; refusing deployment.' >&2
    return 1
  fi
  identity=$(stat -c '%d:%i' "$lock")
  # Opening read-only cannot create a missing file. Linux flock permits an
  # exclusive advisory lock on this descriptor without writing the file.
  exec 9<"$lock" || return 1
  if ! flock -w 1800 9; then
    exec 9<&-
    return 1
  fi
  if [ "$(stat -Lc '%d:%i' /proc/self/fd/9)" != "$identity" ] || \
      [ "$(stat -c '%d:%i' "$lock")" != "$identity" ]; then
    exec 9<&-
    echo 'Original shared deployment lock changed; refusing deployment.' >&2
    return 1
  fi
  RETENTION_PREVIOUS_IDS=()
  local container_id image_id container_ids
  container_ids="$($SUDO docker ps -aq)"
  while IFS= read -r container_id; do
    [ -z "$container_id" ] && continue
    image_id="$($SUDO docker inspect --format '{{.Image}}' "$container_id")"
    RETENTION_PREVIOUS_IDS+=(--protect-image-id "$image_id")
  done <<< "$container_ids"
}

retain_successful_image() {
  local image_ref="$1"
  # The deploy package is writable by deploy. Never execute its Python file as
  # root; the host administrator installs an identical, root-owned copy.
  local trusted_helper="/usr/local/libexec/fangcun/image-retention.py"
  if [ -L "$trusted_helper" ] || [ ! -f "$trusted_helper" ] || \
      [ "$(stat -c '%u:%a' "$trusted_helper" 2>/dev/null)" != "0:755" ] || \
      [ "$(stat -c '%u:%a' "$(dirname "$trusted_helper")" 2>/dev/null)" != "0:755" ] || \
      ! cmp -s "$SCRIPT_DIR/image-retention.py" "$trusted_helper"; then
    echo "::warning::Trusted image retention helper missing, untrusted or differs; skip cleanup." >&2
    return 0
  fi
  local ancestor
  ancestor=$(dirname "$trusted_helper")
  while :; do
    if [ -L "$ancestor" ] || [ "$(stat -c '%u' "$ancestor")" != 0 ] || \
        [ -n "$(find "$ancestor" -maxdepth 0 -perm /022 -print)" ]; then
      echo '::warning::Untrusted image retention parent; skip cleanup.' >&2
      return 0
    fi
    [ "$ancestor" = / ] && break
    ancestor=$(dirname "$ancestor")
  done
  if ! $SUDO /usr/bin/python3 -I "$trusted_helper" \
      --service "${IMAGE_NAME##*/}" --image-ref "$image_ref" --apply --deployment-locked "${RETENTION_PREVIOUS_IDS[@]}"; then
    echo "::warning::Deployment succeeded, but image retention failed; inspect the server retention audit." >&2
  fi
}
