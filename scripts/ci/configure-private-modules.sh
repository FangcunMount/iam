#!/usr/bin/env bash
# Intended for disposable CI jobs. No host Git config or credential store is edited.
set -euo pipefail
set +x
: "${MODULE_READ_TOKEN:?MODULE_READ_TOKEN is required}"
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_ENV:?GITHUB_ENV is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
[[ "$MODULE_READ_TOKEN" != *$'\n'* && "$MODULE_READ_TOKEN" != *$'\r'* ]] || exit 1
umask 077
module_config=$(mktemp "$RUNNER_TEMP/iam-module-auth.XXXXXX")
complete=false
cleanup() { if [[ "$complete" != true ]]; then rm -f -- "$module_config"; fi; }
trap cleanup EXIT
module_basic=$(printf 'x-access-token:%s' "$MODULE_READ_TOKEN" | base64 | tr -d '\r\n')
printf '::add-mask::%s\n' "$module_basic"
# Restrict the header to the one private module, including Git's optional suffix.
# Keep credentials out of URLs, repository config, Go caches and build arguments.
for suffix in '' '.git'; do
  git config --file "$module_config" --add "http.https://github.com/FangcunMount/reliable-messaging${suffix}.extraheader" "AUTHORIZATION: basic $module_basic"
done
{
  printf 'GIT_CONFIG_GLOBAL=%s\n' "$module_config"
  printf 'GOPRIVATE=github.com/FangcunMount/reliable-messaging\n'
  printf 'GONOSUMDB=github.com/FangcunMount/reliable-messaging\n'
  printf 'GIT_TERMINAL_PROMPT=0\n'
} >> "$GITHUB_ENV"
printf 'config-path=%s\n' "$module_config" >> "$GITHUB_OUTPUT"
complete=true
