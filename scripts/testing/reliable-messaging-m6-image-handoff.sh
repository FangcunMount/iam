#!/usr/bin/env bash
set -euo pipefail

# Start two reviewed source images as real processes against disposable
# dependencies. This does not use production credentials or databases.
sdk=${1:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE}
iam=${2:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE}
new_image=${3:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE}
fallback_image=${4:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE FIXTURE_BINARY PROXY_BINARY}
fixture=${5:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE FIXTURE_BINARY PROXY_BINARY}
proxy_binary=${6:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE FIXTURE_BINARY PROXY_BINARY}
probe_binary=${7:?Usage: run.sh SDK_CHECKOUT IAM_CHECKOUT NEW_IMAGE FALLBACK_IMAGE FIXTURE_BINARY PROXY_BINARY PROBE_BINARY}
[[ "$sdk" = /* && -f "$sdk/tests/integration/compose.yaml" && "$iam" = /* && -f "$iam/scripts/testing/reliable-messaging-business-compose.yaml" ]] || {
  echo 'Isolated SDK and IAM Compose fixtures are required' >&2
  exit 1
}
[[ "$fixture" = /* && -x "$fixture" ]] || { echo 'Executable disposable transaction fixture required' >&2; exit 1; }
[[ "$proxy_binary" = /* && -x "$proxy_binary" ]] || { echo 'Executable disposable NSQ proxy required' >&2; exit 1; }
[[ "$probe_binary" = /* && -x "$probe_binary" ]] || { echo 'Executable disposable NSQ probe required' >&2; exit 1; }
new_sha=${RM_NEW_SHA:?Exact new image source SHA required}
fallback_sha=${RM_FALLBACK_SHA:?Exact fallback image source SHA required}
[[ "$new_sha" =~ ^[0-9a-f]{40}$ && "$fallback_sha" =~ ^[0-9a-f]{40}$ ]]
[[ $(docker image inspect "$new_image" --format '{{.Os}}/{{.Architecture}} {{.Config.User}} {{index .Config.Labels "org.opencontainers.image.revision"}}') == "linux/amd64 www $new_sha" ]]
[[ $(docker image inspect "$fallback_image" --format '{{.Os}}/{{.Architecture}} {{.Config.User}} {{index .Config.Labels "org.opencontainers.image.revision"}}') == "linux/amd64 www $fallback_sha" ]]

project="rm-iam-image-${GITHUB_RUN_ID:-local}-$$"
network="${project}_default"
app="${project}-apiserver"
redis="${project}-redis"
proxy="${project}-nsq-proxy"
keys="${project}-keys"
compose=(docker compose --project-name "$project" --file "$sdk/tests/integration/compose.yaml" --file "$iam/scripts/testing/reliable-messaging-business-compose.yaml")
scratch=$(mktemp -d "${RUNNER_TEMP:-/tmp}/${project}.XXXXXX")
cleanup() {
  result=$?
  trap - EXIT
  if ((result != 0)); then
    docker logs --tail 60 "$app" 2>/dev/null || true
    "${compose[@]}" logs --tail 30 --no-color mysql nsqd nsqlookupd 2>/dev/null || true
  fi
  docker rm -f "$app" "$redis" "$proxy" >/dev/null 2>&1 || true
  docker volume rm "$keys" >/dev/null 2>&1 || true
  "${compose[@]}" down --volumes --remove-orphans --timeout 10 >/dev/null || result=1
  rm -rf -- "$scratch"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# The fixture network has no external egress. Reuse the pinned local Redis
# fixture when present; pull only if absent.
docker image inspect redis:7-alpine >/dev/null 2>&1 || docker pull redis:7-alpine >/dev/null
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqlookupd nsqd
docker run -d --name "$redis" --network "$network" redis:7-alpine >/dev/null
for _ in {1..30}; do
  if docker exec "$redis" redis-cli ping 2>/dev/null | grep -qx PONG; then break; fi
  sleep 1
done
docker exec "$redis" redis-cli ping | grep -qx PONG
"${compose[@]}" exec -T mysql mysql -uroot -e 'CREATE DATABASE iam'

# Disposable mTLS material keeps the bundled production-style gRPC config
# active without borrowing serverB certificates.
mkdir -p "$scratch/grpc/ca" "$scratch/grpc/server"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
  -keyout "$scratch/grpc/ca/ca.key" -out "$scratch/grpc/ca/ca-chain.crt" \
  -subj '/CN=iam-image-fixture-ca' >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes \
  -keyout "$scratch/grpc/server/iam-apiserver.key" \
  -out "$scratch/grpc/server/iam-apiserver.csr" \
  -subj '/CN=iam-apiserver' >/dev/null 2>&1
openssl x509 -req -days 1 \
  -in "$scratch/grpc/server/iam-apiserver.csr" \
  -CA "$scratch/grpc/ca/ca-chain.crt" -CAkey "$scratch/grpc/ca/ca.key" \
  -CAcreateserial -out "$scratch/grpc/server/iam-apiserver.crt" \
  -extfile <(printf 'subjectAltName=DNS:iam-apiserver,DNS:localhost,IP:127.0.0.1\n') >/dev/null 2>&1
chmod 755 "$scratch" "$scratch/grpc" "$scratch/grpc/ca" "$scratch/grpc/server"
chmod 644 "$scratch/grpc/ca/ca-chain.crt" "$scratch/grpc/server/iam-apiserver.crt" "$scratch/grpc/server/iam-apiserver.key"
docker volume create "$keys" >/dev/null

start_app() {
  local image=$1 mode=$2 sdk_consumer=$3 nsqd_address=${4:-nsqd:4150} retry_delay=${5:-10s}
  docker run -d --name "$app" --network "$network" \
    --mount "type=volume,source=$keys,target=/app/data/keys" \
    --mount "type=bind,source=$scratch/grpc/ca/ca-chain.crt,target=/etc/iam/grpc/ca/ca-chain.crt,readonly" \
    --mount "type=bind,source=$scratch/grpc/server/iam-apiserver.crt,target=/etc/iam/grpc/server/iam-apiserver.crt,readonly" \
    --mount "type=bind,source=$scratch/grpc/server/iam-apiserver.key,target=/etc/iam/grpc/server/iam-apiserver.key,readonly" \
    -e IAM_APISERVER_MYSQL_HOST=mysql:3306 \
    -e IAM_APISERVER_MYSQL_USERNAME=root \
    -e IAM_APISERVER_MYSQL_PASSWORD= \
    -e IAM_APISERVER_MYSQL_DATABASE=iam \
    -e IAM_APISERVER_REDIS_CACHE_HOST="$redis" \
    -e IAM_APISERVER_REDIS_CACHE_PORT=6379 \
    -e IAM_APISERVER_REDIS_CACHE_USERNAME= \
    -e IAM_APISERVER_REDIS_CACHE_PASSWORD= \
    -e IAM_APISERVER_IDP_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef \
    -e IAM_APISERVER_SEED_MOCK_AUTH_ENABLED=false \
    -e IAM_APISERVER_NSQ_ENABLED=true \
    -e IAM_APISERVER_NSQ_NSQD_ADDR="$nsqd_address" \
    -e IAM_APISERVER_NSQ_LOOKUPD_ADDRS=nsqlookupd:4161 \
    -e IAM_APISERVER_NSQ_NSQD_HTTP_ADDRS=http://nsqd:4151 \
    -e IAM_APISERVER_NSQ_CONSUMER_SDK_ENABLED="$sdk_consumer" \
    -e IAM_APISERVER_EVENTS_RELIABLE_MESSAGING_ENABLED="$mode" \
    -e IAM_APISERVER_EVENTS_OUTBOX_RELAY_RETRY_DELAY="$retry_delay" \
    "$image" >/dev/null
}

wait_http() {
  local port=$1 path=$2
  for _ in {1..120}; do
    if [[ $(docker inspect "$app" --format '{{.State.Running}}' 2>/dev/null) != true ]]; then
      echo "IAM image exited before $path on port $port" >&2
      return 1
    fi
    if docker exec "$app" curl -fsS --max-time 2 "http://127.0.0.1:${port}${path}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "IAM image did not serve $path on port $port" >&2
  return 1
}

# First boot creates the same schema and signing-key material an upgraded
# installation retains. Keep its actual legacy Relay alive until the original
# migration notifications are published; never synthesize their final state.
start_app "$fallback_image" false false
wait_http 9080 /healthz
legacy_total=0
legacy_unfinished=0
for _ in {1..90}; do
  legacy_total=$("${compose[@]}" exec -T mysql mysql -uroot -N -e \
    "SELECT COUNT(*) FROM iam.domain_event_outbox")
  legacy_unfinished=$("${compose[@]}" exec -T mysql mysql -uroot -N -e \
    "SELECT COUNT(*) FROM iam.domain_event_outbox WHERE BINARY status <> 'published'")
  if ((legacy_total >= 2 && legacy_unfinished == 0)); then
    break
  fi
  sleep 1
done
if ((legacy_total < 2 || legacy_unfinished != 0)); then
  echo "legacy Relay did not publish the bootstrap notifications: total=$legacy_total unfinished=$legacy_unfinished" >&2
  exit 1
fi
legacy_nsq_count=$("${compose[@]}" exec -T nsqd wget -qO- http://127.0.0.1:4151/stats?format=json | python3 -c '
import json, sys
topics = json.load(sys.stdin)["topics"]
print(next((t["message_count"] for t in topics if t["topic_name"] == "iam.authz.version.v2"), 0))
')
if ((legacy_nsq_count < legacy_total)); then
  echo "legacy Relay final states lack matching NSQ publishes: rows=$legacy_total nsq=$legacy_nsq_count" >&2
  exit 1
fi
printf 'Legacy Relay published %s original bootstrap notifications; NSQ accepted %s physical messages before handoff.\n' "$legacy_total" "$legacy_nsq_count"
docker stop --time 20 "$app" >/dev/null
docker rm "$app" >/dev/null
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT IF(COUNT(*) = 0, 'drained', 'not-drained') FROM iam.domain_event_outbox WHERE status <> 'published'" | grep -qx drained
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='iam' AND table_name='rm_outbox' AND column_name IN ('failure_count','updated_at')" | grep -qx 2
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT CONCAT(version, ':', dirty + 0) FROM iam.schema_migrations" | grep -qx '40:0'
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='iam' AND table_name='iam_nsq_failure_audit'" | grep -qx 1
"${compose[@]}" exec -T mysql mysql -uroot -e \
  "INSERT INTO iam.iam_nsq_failure_audit (identity_hash,topic,channel_name,application_id,first_transport_id,last_transport_id,metadata_json,metadata_hash,payload,payload_hash,first_cause,last_cause,attempts,source_timestamp_ns,first_seen_unix_ms,last_seen_unix_ms) VALUES (UNHEX(REPEAT('11',32)),'iam.authz.version.v2','iam-policy-sync.previous.1#ephemeral','retained-image-failure','nsq-1','nsq-1','{}',UNHEX(REPEAT('22',32)),'{}',UNHEX(REPEAT('33',32)),'reload failed','reload failed',5,0,1,1)"

start_app "$new_image" true true
wait_http 9080 /readyz
wait_http 9091 /readyz
test "$(docker inspect "$app" --format '{{.State.Running}}')" = true
"${compose[@]}" exec -T nsqd wget -qO- http://127.0.0.1:4151/stats?format=json | python3 -c '
import json, sys
topics = json.load(sys.stdin)["topics"]
channels = [c for t in topics if t["topic_name"] == "iam.authz.version.v2" for c in t["channels"]]
assert any(c["channel_name"].startswith("iam-policy-sync.") and c["channel_name"].endswith("#ephemeral") and c["client_count"] > 0 for c in channels), channels
print("SDK ephemeral policy channel is online")
'
docker stop --time 20 "$app" >/dev/null
docker rm "$app" >/dev/null

# A disposable host fact and valid SDK intent commit in the same MySQL
# transaction while no Relay owns the row. The fallback must fail closed;
# only the compatible image may resume the original identity.
docker run --rm --network "$network" \
  --mount "type=bind,source=$fixture,target=/tmp/iam-m6-handoff-fixture,readonly" \
  --entrypoint /tmp/iam-m6-handoff-fixture \
  -e 'RM_IAM_M6_FIXTURE_DSN=root@tcp(mysql:3306)/iam?parseTime=true&loc=UTC' \
  "$new_image"
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT COUNT(*) FROM iam.m6_handoff_fact WHERE id='m6-standard-handoff-fixture'" | grep -qx 1
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-handoff-fixture'" | grep -qx 'pending:0:0'

start_app "$fallback_image" false false
if wait_http 9080 /healthz; then
  echo 'fallback image incorrectly started with unfinished standard work' >&2
  exit 1
fi
test "$(docker inspect "$app" --format '{{.State.Running}}')" = false
test "$(docker inspect "$app" --format '{{.State.ExitCode}}')" != 0
fallback_log=$(docker logs "$app" 2>&1)
grep -Fq 'message handoff would leave unfinished records without their owner' <<<"$fallback_log"
docker rm "$app" >/dev/null
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-handoff-fixture'" | grep -qx 'pending:0:0'
printf 'Fallback image refused to abandon the pending standard intent.\n'

start_app "$new_image" true true
wait_http 9080 /readyz
standard_state=''
for _ in {1..90}; do
  standard_state=$("${compose[@]}" exec -T mysql mysql -uroot -N -e \
    "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-handoff-fixture'")
  if [[ "$standard_state" = 'published:1:0' ]]; then
    break
  fi
  sleep 1
done
test "$standard_state" = 'published:1:0'
standard_nsq_count=$("${compose[@]}" exec -T nsqd wget -qO- http://127.0.0.1:4151/stats?format=json | python3 -c '
import json, sys
topics = json.load(sys.stdin)["topics"]
print(next((t["message_count"] for t in topics if t["topic_name"] == "iam.authz.version.v2"), 0))
')
((standard_nsq_count >= legacy_nsq_count + 1))
printf 'Compatible image recovered the original standard intent; NSQ count advanced from %s to %s.\n' "$legacy_nsq_count" "$standard_nsq_count"
docker stop --time 20 "$app" >/dev/null
docker rm "$app" >/dev/null

start_app "$fallback_image" false false
wait_http 9080 /readyz
wait_http 9091 /readyz
test "$(docker inspect "$app" --format '{{.State.Running}}')" = true
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT COUNT(*) FROM iam.iam_nsq_failure_audit WHERE application_id='retained-image-failure'" | grep -qx 1
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT CONCAT(version, ':', dirty + 0) FROM iam.schema_migrations" | grep -qx '40:0'
printf 'PASS fallback -> SDK policy subscriber -> fallback image sequence on schema40; retained audit, disposable dependencies, HTTP/gRPC readiness.\n'

# A second committed intent exercises the distinct unknown-confirmation case.
# Only the disposable proxy drops one PUB OK; nsqd still accepts the bytes.
# Keep the retry delay long enough to inspect the first failure before the
# compatible image resumes it. Never let the fallback claim this row.
docker stop --time 20 "$app" >/dev/null
docker rm "$app" >/dev/null
mkdir -p "$scratch/proxy"
chmod 755 "$scratch/proxy"
docker run -d --name "$proxy" --network "$network" \
  --mount "type=bind,source=$proxy_binary,target=/tmp/iam-m6-nsq-proxy,readonly" \
  --mount "type=bind,source=$scratch/proxy,target=/tmp/iam-m6-proxy-state,readonly" \
  --entrypoint /tmp/iam-m6-nsq-proxy \
  -e RM_IAM_M6_PROXY_UPSTREAM=nsqd:4150 \
  -e RM_IAM_M6_PROXY_ARM_FILE=/tmp/iam-m6-proxy-state/arm \
  -e RM_IAM_M6_PROXY_TOPIC=iam.authz.version.v2 \
  "$new_image" >/dev/null
for _ in {1..30}; do
  if docker logs "$proxy" 2>&1 | grep -Fq 'NSQ test proxy ready'; then break; fi
  sleep 1
done
docker logs "$proxy" 2>&1 | grep -Fq 'NSQ test proxy ready'
start_app "$new_image" true true "$proxy:4150" 60s
wait_http 9080 /readyz
"${compose[@]}" exec -T nsqd wget -qO- \
  'http://127.0.0.1:4151/channel/create?topic=iam.authz.version.v2&channel=m6-unknown-proof' >/dev/null
touch "$scratch/proxy/arm"
docker run --rm --network "$network" \
  --mount "type=bind,source=$fixture,target=/tmp/iam-m6-handoff-fixture,readonly" \
  --entrypoint /tmp/iam-m6-handoff-fixture \
  -e 'RM_IAM_M6_FIXTURE_DSN=root@tcp(mysql:3306)/iam?parseTime=true&loc=UTC' \
  -e RM_IAM_M6_FIXTURE_ID=m6-standard-unknown-fixture \
  "$new_image"
for _ in {1..45}; do
  if docker logs "$proxy" 2>&1 | grep -Fq 'DROPPED_PUB_OK_AFTER_BROKER_ACCEPT'; then break; fi
  sleep 1
done
docker logs "$proxy" 2>&1 | grep -Fq 'DROPPED_PUB_OK_AFTER_BROKER_ACCEPT'
unknown_state=''
for _ in {1..30}; do
  unknown_state=$("${compose[@]}" exec -T mysql mysql -uroot -N -e \
    "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-unknown-fixture'")
  if [[ "$unknown_state" = 'retry_wait:1:1' ]]; then break; fi
  sleep 1
done
test "$unknown_state" = 'retry_wait:1:1'
unknown_nsq_count=$("${compose[@]}" exec -T nsqd wget -qO- http://127.0.0.1:4151/stats?format=json | python3 -c '
import json, sys
topics = json.load(sys.stdin)["topics"]
print(next((t["message_count"] for t in topics if t["topic_name"] == "iam.authz.version.v2"), 0))
')
test "$unknown_nsq_count" -eq "$((standard_nsq_count + 1))"
docker kill "$app" >/dev/null
docker rm "$app" >/dev/null
printf 'NSQ accepted the second original intent; IAM retained retry_wait:1:1 after lost PUB OK.\n'

start_app "$fallback_image" false false
if wait_http 9080 /healthz; then
  echo 'fallback image incorrectly started with unknown standard work' >&2
  exit 1
fi
test "$(docker inspect "$app" --format '{{.State.Running}}')" = false
fallback_log=$(docker logs "$app" 2>&1)
grep -Fq 'message handoff would leave unfinished records without their owner' <<<"$fallback_log"
docker rm "$app" >/dev/null
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-unknown-fixture'" | grep -qx 'retry_wait:1:1'

start_app "$new_image" true true
wait_http 9080 /readyz
for _ in {1..90}; do
  unknown_state=$("${compose[@]}" exec -T mysql mysql -uroot -N -e \
    "SELECT CONCAT(state, ':', attempt_count, ':', failure_count) FROM iam.rm_outbox WHERE message_id='m6-standard-unknown-fixture'")
  if [[ "$unknown_state" = 'published:2:1' ]]; then break; fi
  sleep 1
done
test "$unknown_state" = 'published:2:1'
recovered_nsq_count=$("${compose[@]}" exec -T nsqd wget -qO- http://127.0.0.1:4151/stats?format=json | python3 -c '
import json, sys
topics = json.load(sys.stdin)["topics"]
print(next((t["message_count"] for t in topics if t["topic_name"] == "iam.authz.version.v2"), 0))
')
test "$recovered_nsq_count" -eq "$((unknown_nsq_count + 1))"
docker run --rm --network "$network" \
  --mount "type=bind,source=$probe_binary,target=/tmp/iam-m6-nsq-message-probe,readonly" \
  --entrypoint /tmp/iam-m6-nsq-message-probe \
  -e RM_IAM_M6_PROBE_NSQD=nsqd:4150 \
  -e RM_IAM_M6_PROBE_ID=m6-standard-unknown-fixture \
  "$new_image"
printf 'PASS lost-confirmation image handoff: original intent retry_wait:1:1 -> published:2:1; NSQ physical count %s -> %s.\n' "$unknown_nsq_count" "$recovered_nsq_count"
