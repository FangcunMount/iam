#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TEST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/iam-setup-ssh-test.XXXXXX")

cleanup() {
  rm -rf "$TEST_TMP"
}
trap cleanup EXIT HUP INT TERM

FAKE_BIN="$TEST_TMP/bin"
mkdir -p "$FAKE_BIN"

cat >"$FAKE_BIN/ssh-keygen" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
echo '256 SHA256:test deploy-key (ED25519)'
EOF

cat >"$FAKE_BIN/ssh" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail

config=''
print_config=false
while (($#)); do
  case "$1" in
    -F)
      config=$2
      shift 2
      ;;
    -G)
      print_config=true
      shift
      ;;
    -o)
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

host=$(awk '$1 == "HostName" { print $2; exit }' "$config")
if [ "$print_config" = true ]; then
  echo "hostname ${host}"
  exit 0
fi

printf '%s\n' "$host" >>"$FAKE_SSH_ATTEMPTS"
if [ "$host" = "$FAKE_SSH_PRIMARY_HOST" ] && [ "$FAKE_SSH_PRIMARY_OK" = true ]; then
  echo "$FAKE_SSH_REMOTE_HOSTNAME"
  exit 0
fi
if [ "$host" = "$FAKE_SSH_FALLBACK_HOST" ] && [ "$FAKE_SSH_FALLBACK_OK" = true ]; then
  echo "$FAKE_SSH_REMOTE_HOSTNAME"
  exit 0
fi
exit 255
EOF
chmod +x "$FAKE_BIN/ssh-keygen" "$FAKE_BIN/ssh"

run_setup() {
  case_name=$1
  primary_ok=$2
  fallback_ok=$3
  case_tmp="$TEST_TMP/$case_name"
  mkdir -p "$case_tmp"
  : >"$case_tmp/github-env"
  : >"$case_tmp/attempts"
  PATH="$FAKE_BIN:$PATH" \
    RUNNER_TEMP="$case_tmp" \
    GITHUB_RUN_ID=123 \
    GITHUB_ENV="$case_tmp/github-env" \
    RUNNER_SSH_KEY=dummy-key \
    RUNNER_SSH_HOST=public.example.test \
    RUNNER_SSH_FALLBACK_HOST=100.64.0.2 \
    RUNNER_SSH_USER=deploy \
    RUNNER_SSH_PORT=22 \
    RUNNER_SSH_ALIAS=deploy-target \
    DEPLOY_NODE_HOSTNAME=serverB \
    FAKE_SSH_ATTEMPTS="$case_tmp/attempts" \
    FAKE_SSH_PRIMARY_HOST=public.example.test \
    FAKE_SSH_FALLBACK_HOST=100.64.0.2 \
    FAKE_SSH_PRIMARY_OK="$primary_ok" \
    FAKE_SSH_FALLBACK_OK="$fallback_ok" \
    FAKE_SSH_REMOTE_HOSTNAME=serverB \
    bash "$SCRIPT_DIR/setup-runner-ssh.sh"
}

run_setup primary true true >"$TEST_TMP/primary.log" 2>&1
grep -q '^RUNNER_SSH_HOST=public.example.test$' "$TEST_TMP/primary/github-env"
[ "$(cat "$TEST_TMP/primary/attempts")" = 'public.example.test' ]

run_setup fallback false true >"$TEST_TMP/fallback.log" 2>&1
grep -q '^RUNNER_SSH_HOST=100.64.0.2$' "$TEST_TMP/fallback/github-env"
[ "$(cat "$TEST_TMP/fallback/attempts")" = "$(printf 'public.example.test\n100.64.0.2')" ]
grep -q 'falling back to 100.64.0.2' "$TEST_TMP/fallback.log"

if run_setup unavailable false false >"$TEST_TMP/unavailable.log" 2>&1; then
  echo 'expected setup-runner-ssh.sh to fail when both targets are unavailable' >&2
  exit 1
fi
grep -q 'neither primary nor fallback SSH target passed hostname verification' "$TEST_TMP/unavailable.log"

echo 'setup-runner-ssh primary and fallback tests passed'
