#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ENV_FILE="$(mktemp -t ocm-e2e-env-XXXXXX)"

cleanup() {
  "${SCRIPT_DIR}/down.sh"
  rm -f "${ENV_FILE}"
}
trap cleanup EXIT

"${SCRIPT_DIR}/up.sh" >"${ENV_FILE}"
set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

cd "${REPO_ROOT}"
go test -tags e2e -timeout 30m -v ./test/e2e/...
