#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SPECTRAL_IMAGE="${SPECTRAL_IMAGE:-stoplight/spectral:latest}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required to run spectral lint" >&2
  exit 1
fi

# 默认规则集：本地 extends spectral:oas
RULESET="${SPECTRAL_RULESET:-/workspace/scripts/spectral.yaml}"
echo "Running spectral lint with ruleset ${RULESET} ..."
docker run --rm -v "${ROOT_DIR}:/workspace" -w /workspace "${SPECTRAL_IMAGE}" \
  lint -r "${RULESET}" api/rest/*.yaml

echo "Comparing swagger definitions with OpenAPI components..."
python3 "${ROOT_DIR}/scripts/check-openapi-contracts.py"

echo "Comparing routes (swagger paths) with OpenAPI paths..."
python3 "${ROOT_DIR}/scripts/check-route-contracts.py"

echo "OpenAPI validation completed."
