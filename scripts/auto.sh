#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1
export GOCACHE="${GOCACHE:-/tmp/dataguardian-go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/dataguardian-go-mod}"

DB_STATUS="FAILED"
BACKEND_STATUS="FAILED"
FRONTEND_STATUS="FAILED"
GO_TESTS_STATUS="FAILED"
E2E_TESTS_STATUS="FAILED"

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

http_ok() {
  local url="$1"
  command -v curl >/dev/null 2>&1 && curl -fsS "$url" >/dev/null 2>&1
}

container_running() {
  docker inspect -f '{{.State.Running}}' dataguardian_db 2>/dev/null | grep -qx true
}

printf "\n== Diagnostics ==\n"
if "$ROOT_DIR/scripts/doctor.sh"; then
  printf "Diagnostics: OK\n"
else
  printf "Diagnostics: FAILED\n"
fi

printf "\n== Starting services ==\n"
if "$ROOT_DIR/scripts/up.sh"; then
  if container_running; then
    DB_STATUS="OK"
  fi
  if http_ok "http://localhost:8000/health"; then
    BACKEND_STATUS="OK"
  fi
  if http_ok "http://localhost:3000/login"; then
    FRONTEND_STATUS="OK"
  fi
else
  printf "Service startup failed.\n"
fi

printf "\n== Backend tests ==\n"
if run_go_tests; then
  GO_TESTS_STATUS="PASSED"
fi

printf "\n== E2E tests ==\n"
if pytest_path="$(pytest_bin)"; then
  if "$pytest_path" tests/e2e -s; then
    E2E_TESTS_STATUS="PASSED"
  fi
else
  warn_pytest_missing
  E2E_TESTS_STATUS="SKIPPED"
fi

printf "\n== Summary ==\n"
printf "DB: %s\n" "$DB_STATUS"
printf "Backend: %s\n" "$BACKEND_STATUS"
printf "Frontend: %s\n" "$FRONTEND_STATUS"
printf "Go tests: %s\n" "$GO_TESTS_STATUS"
printf "E2E tests: %s\n" "$E2E_TESTS_STATUS"

if [ "$DB_STATUS" = "OK" ] &&
  [ "$BACKEND_STATUS" = "OK" ] &&
  [ "$FRONTEND_STATUS" = "OK" ] &&
  [ "$GO_TESTS_STATUS" = "PASSED" ] &&
  { [ "$E2E_TESTS_STATUS" = "PASSED" ] || [ "$E2E_TESTS_STATUS" = "SKIPPED" ]; }; then
  exit 0
fi

exit 1
