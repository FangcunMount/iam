#!/usr/bin/env bash
set -euo pipefail

# Start the already-published image as a real process against disposable
# dependencies. No production credentials, hosts or persistent databases.
sdk=${1:?Usage: reliable-messaging-fallback-image-startup.sh SDK_CHECKOUT IMAGE}
image=${2:?Usage: reliable-messaging-fallback-image-startup.sh SDK_CHECKOUT IMAGE}
[[ "$sdk" = /* && -f "$sdk/tests/integration/compose.yaml" ]] || {
  echo 'SDK v0.2.1 isolated Compose fixture is required' >&2
  exit 1
}

project="rm-iam-image-${GITHUB_RUN_ID:-local}-$$"
network="${project}_default"
app="${project}-apiserver"
redis="${project}-redis"
keys="${project}-keys"
compose=(docker compose --project-name "$project" --file "$sdk/tests/integration/compose.yaml")
scratch=$(mktemp -d "${RUNNER_TEMP:-/tmp}/${project}.XXXXXX")
cleanup() {
  result=$?
  trap - EXIT
  if ((result != 0)); then
    docker logs --tail 60 "$app" 2>/dev/null || true
    "${compose[@]}" logs --tail 30 --no-color mysql nsqd 2>/dev/null || true
  fi
  docker rm -f "$app" "$redis" >/dev/null 2>&1 || true
  docker volume rm "$keys" >/dev/null 2>&1 || true
  "${compose[@]}" down --volumes --remove-orphans --timeout 10 >/dev/null || result=1
  rm -rf -- "$scratch"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# The fixture network has no external egress; obtain images before joining it.
docker pull redis:7-alpine >/dev/null
"${compose[@]}" up -d --wait --wait-timeout 180 mysql nsqd
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
  local mode=$1
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
    -e IAM_APISERVER_NSQ_NSQD_ADDR=nsqd:4150 \
    -e IAM_APISERVER_EVENTS_RELIABLE_MESSAGING_ENABLED="$mode" \
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
# installation retains. Drain only this disposable fixture's legacy rows.
start_app false
wait_http 9080 /healthz
docker stop --time 20 "$app" >/dev/null
docker rm "$app" >/dev/null
"${compose[@]}" exec -T mysql mysql -uroot -e \
  "UPDATE iam.domain_event_outbox SET status='published' WHERE status <> 'published'"
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT IF(COUNT(*) = 0, 'drained', 'not-drained') FROM iam.domain_event_outbox WHERE status <> 'published'" | grep -qx drained
"${compose[@]}" exec -T mysql mysql -uroot -N -e \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='iam' AND table_name='rm_outbox' AND column_name IN ('failure_count','updated_at')" | grep -qx 2

start_app true
wait_http 9080 /readyz
wait_http 9091 /readyz
test "$(docker inspect "$app" --format '{{.State.Running}}')" = true
printf 'Published fallback image started with schema39, disposable MySQL/Redis/NSQ and gRPC mTLS; HTTP and gRPC health listeners are ready.\n'
