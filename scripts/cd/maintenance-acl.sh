#!/usr/bin/env bash
# A host-owned, release-bound override keeps an active write freeze across deploy.
# Remove the directory only after the coordinated cutover has been verified.
preserve_maintenance_acl() (
  local freeze_dir="$1" package_dir="$2" release_sha="$3"
  if ! $SUDO test -e "$freeze_dir"; then
    return 0
  fi
  local file expected actual bound_release staging
  for file in "$freeze_dir" "$freeze_dir/grpc_acl.yaml" "$freeze_dir/sha256" "$freeze_dir/release_sha"; do
    if $SUDO test -L "$file"; then
      echo 'Maintenance ACL must not use symbolic links.' >&2
      return 1
    fi
  done
  for file in grpc_acl.yaml sha256 release_sha; do
    if ! $SUDO test -f "$freeze_dir/$file"; then
      echo 'Maintenance ACL is incomplete; refusing deployment.' >&2
      return 1
    fi
  done
  # The deployment account can sync protected files, but cannot run arbitrary
  # root readers. Inspect a private, user-owned copy using unprivileged tools.
  umask 077
  staging=$(mktemp -d "$package_dir/.maintenance-acl.XXXXXX")
  trap 'rm -rf -- "$staging"' EXIT
  $SUDO rsync --chmod=u=rw,go= "$freeze_dir/grpc_acl.yaml" "$freeze_dir/sha256" "$freeze_dir/release_sha" "$staging/"
  $SUDO chown "$(id -u):$(id -g)" "$staging/grpc_acl.yaml" "$staging/sha256" "$staging/release_sha"
  bound_release=$(cat "$staging/release_sha")
  if [ "$bound_release" != "$release_sha" ]; then
    echo 'Maintenance ACL targets a different release; refusing deployment.' >&2
    return 1
  fi
  expected=$(cat "$staging/sha256")
  actual=$(sha256sum "$staging/grpc_acl.yaml")
  actual=${actual%% *}
  if [ -z "$expected" ] || [ "$actual" != "$expected" ]; then
    echo 'Maintenance ACL checksum mismatch; refusing deployment.' >&2
    return 1
  fi
  cp "$staging/grpc_acl.yaml" "$package_dir/configs/grpc_acl.yaml"
  echo 'Preserving the verified maintenance ACL for this release.'
)
