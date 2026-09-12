#!/usr/bin/env bash
# A host-owned, release-bound override keeps an active write freeze across deploy.
# Remove the directory only after the coordinated cutover has been verified.
preserve_maintenance_acl() {
  local freeze_dir="$1" package_dir="$2" release_sha="$3"
  if ! $SUDO test -e "$freeze_dir"; then
    return 0
  fi
  local file expected actual bound_release
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
  bound_release=$($SUDO cat "$freeze_dir/release_sha")
  if [ "$bound_release" != "$release_sha" ]; then
    echo 'Maintenance ACL targets a different release; refusing deployment.' >&2
    return 1
  fi
  expected=$($SUDO cat "$freeze_dir/sha256")
  actual=$($SUDO sha256sum "$freeze_dir/grpc_acl.yaml")
  actual=${actual%% *}
  if [ -z "$expected" ] || [ "$actual" != "$expected" ]; then
    echo 'Maintenance ACL checksum mismatch; refusing deployment.' >&2
    return 1
  fi
  $SUDO cp "$freeze_dir/grpc_acl.yaml" "$package_dir/configs/grpc_acl.yaml"
  echo 'Preserving the verified maintenance ACL for this release.'
}
