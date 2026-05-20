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

if ! command -v docker >/dev/null 2>&1; then
  log "Docker is not installed or not on PATH."
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "Docker is not running."
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  log "curl is required for readiness checks in this helper script."
  log "Install curl, or use: docker compose up --build"
  exit 1
fi

if port_in_use "$BACKEND_PORT" && ! http_ok "http://localhost:$BACKEND_PORT/health"; then
  log "backend port $BACKEND_PORT is already in use, but health check failed."
  log "Run ./scripts/doctor.sh to inspect the process."
  exit 1
fi

if port_in_use "$FRONTEND_PORT" && ! http_ok "http://localhost:$FRONTEND_PORT/login"; then
  log "frontend port $FRONTEND_PORT is already in use, but login page check failed."
  log "Run ./scripts/doctor.sh to inspect the process."
  exit 1
fi

log "starting Docker Compose services: db backend-go frontend"
compose up -d --build db backend-go
compose up -d --build --force-recreate frontend

wait_for_http "backend" "http://localhost:$BACKEND_PORT/health" 45
wait_for_http "frontend" "http://localhost:$FRONTEND_PORT/login" 60

log "system ready"
log "open app: http://localhost:$FRONTEND_PORT"
log "backend health: http://localhost:$BACKEND_PORT/health"
log "local demo users: admin / admin123 or test / test123"
