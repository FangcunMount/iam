#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SPECTRAL_IMAGE="${SPECTRAL_IMAGE:-stoplight/spectral:6.15.0}"

# 默认规则集：本地 extends spectral:oas
RULESET_REL="${SPECTRAL_RULESET:-scripts/spectral.yaml}"
echo "Running spectral lint with ruleset ${RULESET_REL} ..."
SPECTRAL_REPORT="$(mktemp)"
trap 'rm -f "${SPECTRAL_REPORT}"' EXIT
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  docker run --rm -v "${ROOT_DIR}:/workspace" -w /workspace "${SPECTRAL_IMAGE}" \
    lint -r "/workspace/${RULESET_REL}" --fail-severity error --format json api/rest/*.yaml \
    >"${SPECTRAL_REPORT}"
elif command -v npx >/dev/null 2>&1; then
  echo "docker is unavailable; using pinned @stoplight/spectral-cli@6.15.0"
  npx --yes @stoplight/spectral-cli@6.15.0 \
    lint -r "${ROOT_DIR}/${RULESET_REL}" --fail-severity error --format json \
    "${ROOT_DIR}"/api/rest/*.yaml >"${SPECTRAL_REPORT}"
else
  echo "docker or npx is required to run spectral lint" >&2
  exit 1
fi
python3 "${ROOT_DIR}/scripts/check-spectral-baseline.py" \
  "${SPECTRAL_REPORT}" "${ROOT_DIR}/scripts/spectral-warnings-baseline.json"

echo "Comparing swagger definitions with OpenAPI components..."
python3 "${ROOT_DIR}/scripts/check-openapi-contracts.py"

echo "Comparing routes (swagger paths) with OpenAPI paths..."
python3 "${ROOT_DIR}/scripts/check-route-contracts.py"

echo "Comparing registered Gin routes with OpenAPI paths..."
"${GO_BIN:-go}" test ./internal/apiserver/transport/rest \
  -run '^TestRouterOpenAPIContractCoversRegisteredPublicRoutes$' -count=1

echo "OpenAPI validation completed."
