#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export GOCACHE="${GOCACHE:-/tmp/dataguardian-go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/dataguardian-go-mod}"

pytest_bin() {
  if [ -n "${PYTEST_BIN:-}" ]; then
    printf '%s\n' "$PYTEST_BIN"
    return 0
  fi

  if command -v pytest >/dev/null 2>&1; then
    command -v pytest
    return 0
  fi

  return 1
}

warn_pytest_missing() {
  printf '[WARN] pytest not found. Skipping E2E tests.\n'
}

(
  cd "$ROOT_DIR/backend-go"
  go test ./...
)

(
  cd "$ROOT_DIR/frontend"
  npm run build
)

if pytest_path="$(pytest_bin)"; then
  "$pytest_path" "$ROOT_DIR/tests/test_backend_mode_contract.py"
  "$pytest_path" "$ROOT_DIR/tests/e2e"
else
  warn_pytest_missing
fi
