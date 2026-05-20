#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log() {
  printf '[release-check] %s\n' "$1"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "$1 is required."
    exit 1
  fi
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    log "Docker Compose is required."
    exit 1
  fi
}

check_file() {
  if [ ! -f "$1" ]; then
    log "required file is missing: $1"
    exit 1
  fi
}

check_shell() {
  check_file "$1"
  bash -n "$1"
}

need bash
need docker
need curl

log "checking required release files"
check_file README.md
check_file CHANGELOG.md
check_file BACKLOG.md
check_file SECURITY.md
check_file docker-compose.yml
check_file .env.example

log "checking shell helper syntax"
check_shell scripts/release-check.sh
check_shell start.sh
check_shell scripts/up.sh
check_shell scripts/smoke.sh

log "checking Docker Compose configuration"
compose config --quiet

log "preflight passed"
log "next: docker compose up --build"
log "then: ./scripts/smoke.sh"
