#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_LOG="$ROOT_DIR/.logs/backend.log"
FRONTEND_LOG="$ROOT_DIR/.logs/frontend.log"
BACKEND_PID=""
FRONTEND_PID=""

mkdir -p "$ROOT_DIR/.logs"

log() {
  printf '[START] %s\n' "$1"
}

port_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -ti "tcp:$port" || true
  elif command -v fuser >/dev/null 2>&1; then
    fuser "$port/tcp" 2>/dev/null || true
  fi
}

free_port() {
  local port="$1"
  local pids
  pids="$(port_pids "$port")"
  if [ -n "$pids" ]; then
    log "Stopping process on port $port"
    kill $pids 2>/dev/null || true
    sleep 1
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

start_postgres() {
  log "Starting PostgreSQL with Docker Compose"
  docker compose up -d
}

start_backend() {
  free_port 8000
  log "Starting backend on http://localhost:8000"
  (
    cd "$ROOT_DIR/backend"
    PYTHONPATH="$ROOT_DIR/backend" ../.venv/bin/uvicorn app.main:app --host 127.0.0.1 --port 8000
  ) >"$BACKEND_LOG" 2>&1 &
  BACKEND_PID="$!"
  wait_for_url "http://localhost:8000/health" "Backend"
}

start_frontend() {
  free_port 3000
  log "Starting frontend on http://localhost:3000"
  (
    cd "$ROOT_DIR/frontend"
    npm run dev -- -H 127.0.0.1 -p 3000
  ) >"$FRONTEND_LOG" 2>&1 &
  FRONTEND_PID="$!"
  wait_for_url "http://localhost:3000/login" "Frontend"
}

stop_started_services() {
  if [ -n "$FRONTEND_PID" ]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
  fi
  if [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
}

manual_mode() {
  trap stop_started_services INT TERM
  start_postgres
  start_backend
  start_frontend
  log "Manual mode ready"
  log "Backend log: $BACKEND_LOG"
  log "Frontend log: $FRONTEND_LOG"
  wait
}

auto_mode() {
  trap stop_started_services EXIT INT TERM
  log "Running security checks"
  "$ROOT_DIR/security/run_security_checks.sh"
  start_postgres
  start_backend
  start_frontend
  log "Running audit"
  "$ROOT_DIR/security/audit.sh"
  log "Running backend tests"
  "$ROOT_DIR/.venv/bin/pytest" "$ROOT_DIR/backend/tests"
  log "Running E2E tests"
  "$ROOT_DIR/.venv/bin/pytest" "$ROOT_DIR/tests/e2e"
  log "Automatic mode completed"
}

case "${1:-}" in
  manual)
    manual_mode
    ;;
  auto)
    auto_mode
    ;;
  *)
    printf 'Usage: %s manual|auto\n' "$0"
    exit 2
    ;;
esac
