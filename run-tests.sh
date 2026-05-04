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

run_go_tests() {
  if go version 2>/dev/null | grep -Eq 'go1\.(25|2[6-9]|[3-9][0-9])'; then
    (cd "$ROOT_DIR/backend-go" && go test ./...)
    return
  fi

  if docker compose version >/dev/null 2>&1; then
    docker compose run --rm backend-go go test ./...
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose run --rm backend-go go test ./...
  else
    printf 'Go 1.25 or Docker Compose is required to run backend tests.\n' >&2
    return 1
  fi
}

run_go_tests

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
