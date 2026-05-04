#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

LOG_DIR="$ROOT_DIR/.logs"
BACKEND_PORT=8000
FRONTEND_PORT=3000

mkdir -p "$LOG_DIR"

log() {
  printf "[up] %s\n" "$1"
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    log "Docker Compose is not available."
    exit 1
  fi
}

http_ok() {
  local url="$1"
  command -v curl >/dev/null 2>&1 && curl -fsS "$url" >/dev/null 2>&1
}

port_in_use() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi

  if command -v ss >/dev/null 2>&1; then
    ss -ltn "sport = :$port" 2>/dev/null | grep -q ":$port"
    return
  fi

  return 1
}

wait_for_http() {
  local name="$1"
  local url="$2"
  local seconds="$3"

  for _ in $(seq 1 "$seconds"); do
    if http_ok "$url"; then
      log "$name running"
      return 0
    fi
    sleep 1
  done

  log "$name did not become ready at $url"
  return 1
}

run_detached() {
  local log_file="$1"
  shift

  if command -v setsid >/dev/null 2>&1; then
    setsid "$@" >"$log_file" 2>&1 < /dev/null &
  else
    nohup "$@" >"$log_file" 2>&1 < /dev/null &
  fi
}

start_backend() {
  if http_ok "http://localhost:$BACKEND_PORT/health"; then
    log "backend running"
    return 0
  fi

  if port_in_use "$BACKEND_PORT"; then
    log "backend port $BACKEND_PORT is already in use, but health check failed."
    log "Run ./scripts/doctor.sh to inspect the process."
    return 1
  fi

  local uvicorn_cmd="uvicorn"
  if [ -x "$ROOT_DIR/.venv/bin/uvicorn" ]; then
    uvicorn_cmd="$ROOT_DIR/.venv/bin/uvicorn"
  fi

  log "starting backend on port $BACKEND_PORT"
  (
    cd "$ROOT_DIR/backend"
    run_detached "$LOG_DIR/backend.log" "$uvicorn_cmd" app.main:app --reload --port "$BACKEND_PORT"
  )

  wait_for_http "backend" "http://localhost:$BACKEND_PORT/health" 30
}

start_frontend() {
  if http_ok "http://localhost:$FRONTEND_PORT/login"; then
    log "frontend running"
    return 0
  fi

  if port_in_use "$FRONTEND_PORT"; then
    log "frontend port $FRONTEND_PORT is already in use, but login page check failed."
    log "Run ./scripts/doctor.sh to inspect the process."
    return 1
  fi

  log "starting frontend on port $FRONTEND_PORT"
  (
    cd "$ROOT_DIR/frontend"
    run_detached "$LOG_DIR/frontend.log" npm run dev
  )

  wait_for_http "frontend" "http://localhost:$FRONTEND_PORT/login" 45
}

if ! command -v docker >/dev/null 2>&1; then
  log "Docker is not installed or not on PATH."
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "Docker is not running."
  exit 1
fi

log "starting PostgreSQL"
compose up -d
log "DB running"

start_backend
start_frontend

log "system ready"
