#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE=""
VISUAL=""

mkdir -p "$ROOT_DIR/.logs"

log() {
  printf '[START] %s\n' "$1"
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    log "Docker Compose is required"
    exit 1
  fi
}

usage() {
  printf 'Usage: %s [manual|auto] [--visual]\n' "$0"
}

parse_args() {
  for arg in "$@"; do
    case "$arg" in
      manual|auto)
        if [ -n "$MODE" ]; then
          usage
          exit 2
        fi
        MODE="$arg"
        ;;
      --visual)
        VISUAL="--visual"
        ;;
      *)
        usage
        exit 2
        ;;
    esac
  done

  if [ -z "$MODE" ]; then
    MODE="manual"
  fi
}

port_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -ti "tcp:$port" || true
  elif command -v fuser >/dev/null 2>&1; then
    fuser "$port/tcp" 2>/dev/null || true
  fi
}

describe_port_owner() {
  local port="$1"
  local pids="$2"
  if command -v ps >/dev/null 2>&1; then
    ps -o pid=,comm= -p $pids 2>/dev/null || true
  fi
}

require_port_available_or_dataguardian() {
  local port="$1"
  local url="$2"
  local label="$3"
  local pids
  pids="$(port_pids "$port")"
  if [ -z "$pids" ]; then
    return 0
  fi

  if curl -fsS "$url" >/dev/null 2>&1; then
    log "$label already appears to be running on port $port; reusing it"
    return 1
  fi

  log "Port $port is already in use by a non-DataGuardian process:"
  describe_port_owner "$port" "$pids"
  log "Stop that process manually, then rerun ./start.sh ${MODE:-manual}"
  exit 1
}

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    log "Docker is required"
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    log "Docker is not running"
    exit 1
  fi
}

require_curl() {
  if ! command -v curl >/dev/null 2>&1; then
    log "curl is required for readiness checks in this helper script"
    log "Install curl, or use: docker compose up --build"
    exit 1
  fi
}

wait_for_url() {
  local url="$1"
  local label="$2"
  for _ in $(seq 1 45); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$label is ready"
      return 0
    fi
    sleep 1
  done
  log "$label did not become ready"
  return 1
}

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
  log "[WARN] pytest not found. Skipping E2E tests."
}

print_ready_summary() {
  log "DataGuardian is ready"
  log "Open app: http://localhost:3000"
  log "Backend health: http://localhost:8000/health"
  log "Local demo users: admin / admin123 or test / test123"
}

run_go_tests() {
  compose run --rm backend-go-test go test ./...
}

start_services() {
  require_docker
  require_curl
  require_port_available_or_dataguardian 8000 "http://localhost:8000/health" "Backend" || true
  require_port_available_or_dataguardian 3000 "http://localhost:3000/login" "Frontend" || true

  log "Starting Docker Compose services: db backend-go frontend"
  compose up -d --build db backend-go
  compose up -d --build --force-recreate frontend

  for _ in $(seq 1 30); do
    if compose exec -T db pg_isready -U dataguardian -d dataguardian >/dev/null 2>&1; then
      log "PostgreSQL is ready"
      break
    fi
    sleep 1
  done
  if ! compose exec -T db pg_isready -U dataguardian -d dataguardian >/dev/null 2>&1; then
    log "PostgreSQL did not become ready"
    return 1
  fi

  wait_for_url "http://localhost:8000/health" "Backend"
  wait_for_url "http://localhost:3000/login" "Frontend"
  print_ready_summary
}

manual_mode() {
  start_services
  log "Backend: Go"
  log "Backend log: docker compose logs -f backend-go"
  log "Frontend log: docker compose logs -f frontend"
  compose logs -f backend-go frontend
}

auto_mode() {
  export GOCACHE="${GOCACHE:-/tmp/dataguardian-go-build}"
  export GOMODCACHE="${GOMODCACHE:-/tmp/dataguardian-go-mod}"
  log "Running security checks"
  "$ROOT_DIR/security/run_security_checks.sh"
  start_services
  log "Running audit"
  "$ROOT_DIR/security/audit.sh"
  log "Running Go backend tests"
  run_go_tests
  log "Running E2E tests"
  if pytest_path="$(pytest_bin)"; then
    if [ "$VISUAL" = "--visual" ]; then
      E2E_HEADLESS=false "$pytest_path" "$ROOT_DIR/tests/e2e" -s
    else
      "$pytest_path" "$ROOT_DIR/tests/e2e"
    fi
  else
    warn_pytest_missing
  fi
  log "Automatic mode completed"
}

parse_args "$@"

case "$MODE" in
  manual)
    manual_mode
    ;;
  auto)
    auto_mode
    ;;
  *)
    usage
    exit 2
    ;;
esac
