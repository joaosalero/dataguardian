#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1

DB_STATUS="FAILED"
BACKEND_STATUS="FAILED"
FRONTEND_STATUS="FAILED"
BACKEND_TESTS_STATUS="FAILED"
E2E_TESTS_STATUS="FAILED"

pytest_cmd() {
  if [ -x "$ROOT_DIR/.venv/bin/pytest" ]; then
    "$ROOT_DIR/.venv/bin/pytest" "$@"
  else
    pytest "$@"
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
if pytest_cmd backend/tests; then
  BACKEND_TESTS_STATUS="PASSED"
fi

printf "\n== E2E tests ==\n"
if pytest_cmd tests/e2e -s; then
  E2E_TESTS_STATUS="PASSED"
fi

printf "\n== Summary ==\n"
printf "DB: %s\n" "$DB_STATUS"
printf "Backend: %s\n" "$BACKEND_STATUS"
printf "Frontend: %s\n" "$FRONTEND_STATUS"
printf "Backend tests: %s\n" "$BACKEND_TESTS_STATUS"
printf "E2E tests: %s\n" "$E2E_TESTS_STATUS"

if [ "$DB_STATUS" = "OK" ] &&
  [ "$BACKEND_STATUS" = "OK" ] &&
  [ "$FRONTEND_STATUS" = "OK" ] &&
  [ "$BACKEND_TESTS_STATUS" = "PASSED" ] &&
  [ "$E2E_TESTS_STATUS" = "PASSED" ]; then
  exit 0
fi

exit 1

