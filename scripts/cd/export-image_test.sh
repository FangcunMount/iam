#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TEST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/iam-export-image-test.XXXXXX")

cleanup() {
  rm -rf "$TEST_TMP"
}
trap cleanup EXIT HUP INT TERM

FAKE_BIN="$TEST_TMP/bin"
FIXTURE_DIR="$TEST_TMP/fixture"
mkdir -p "$FAKE_BIN" "$FIXTURE_DIR"
printf '{}\n' >"$FIXTURE_DIR/manifest.json"

cat >"$FAKE_BIN/docker" <<'EOF'
#!/usr/bin/env sh
set -eu

case "${1:-}" in
  pull)
    count=0
    if [ -f "$FAKE_DOCKER_STATE" ]; then
      count=$(cat "$FAKE_DOCKER_STATE")
    fi
    count=$((count + 1))
    printf '%s\n' "$count" >"$FAKE_DOCKER_STATE"
    if [ "$count" -le "$FAKE_DOCKER_PULL_FAILURES" ]; then
      echo "simulated pull failure ${count}" >&2
      exit 1
    fi
    ;;
  save)
    shift
    output=''
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -o)
          output=$2
          shift 2
          ;;
        *)
          shift
          ;;
      esac
    done
    [ -n "$output" ] || exit 2
    tar -cf "$output" -C "$FAKE_DOCKER_FIXTURE_DIR" manifest.json
    ;;
  *)
    echo "unexpected docker command: ${1:-}" >&2
    exit 2
    ;;
esac
EOF

cat >"$FAKE_BIN/sleep" <<'EOF'
#!/usr/bin/env sh
set -eu
printf '%s\n' "$1" >>"$FAKE_SLEEP_STATE"
EOF
chmod +x "$FAKE_BIN/docker" "$FAKE_BIN/sleep"

run_export() {
  failures=$1
  output=$2
  : >"$FAKE_DOCKER_STATE"
  : >"$FAKE_SLEEP_STATE"
  PATH="$FAKE_BIN:$PATH" \
    SERVICE=apiserver \
    DOCKER_REGISTRY=ghcr.io \
    DOCKER_REPOSITORY=test \
    DEPLOY_SHA=abcdef \
    EXPORT_IMAGE_REGISTRY=acr \
    ALIYUN_ACR_REGISTRY=registry.example.test \
    ALIYUN_ACR_NAMESPACE=test \
    DEPLOY_IMAGE_PACKAGE="$output" \
    PULL_MAX_ATTEMPTS=4 \
    PULL_RETRY_INITIAL_DELAY_SECONDS=5 \
    FAKE_DOCKER_STATE="$FAKE_DOCKER_STATE" \
    FAKE_DOCKER_PULL_FAILURES="$failures" \
    FAKE_DOCKER_FIXTURE_DIR="$FIXTURE_DIR" \
    FAKE_SLEEP_STATE="$FAKE_SLEEP_STATE" \
    "$SCRIPT_DIR/export-image.sh"
}

FAKE_DOCKER_STATE="$TEST_TMP/docker-state"
FAKE_SLEEP_STATE="$TEST_TMP/sleep-state"

success_output="$TEST_TMP/success.tar.gz"
run_export 3 "$success_output" >"$TEST_TMP/success.log" 2>&1
[ "$(cat "$FAKE_DOCKER_STATE")" = '4' ]
[ "$(cat "$FAKE_SLEEP_STATE")" = "$(printf '5\n10\n20')" ]
[ -s "$success_output" ]
gzip -dc "$success_output" | tar -tf - | grep -qx 'manifest.json'

failure_output="$TEST_TMP/failure.tar.gz"
if run_export 4 "$failure_output" >"$TEST_TMP/failure.log" 2>&1; then
  echo "expected export-image.sh to fail after retry exhaustion" >&2
  exit 1
fi
[ "$(cat "$FAKE_DOCKER_STATE")" = '4' ]
[ "$(cat "$FAKE_SLEEP_STATE")" = "$(printf '5\n10\n20')" ]
[ ! -e "$failure_output" ]
grep -q 'Pull failed after 4 attempts' "$TEST_TMP/failure.log"

echo "export-image retry tests passed"
