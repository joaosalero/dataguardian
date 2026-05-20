#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_URL="http://localhost:3000"

log() {
  printf '[DataGuardian] %s\n' "$1"
}

help_recovery() {
  printf '\n'
  log "Startup did not complete."
  log "Try these recovery steps:"
  log "1. Make sure Docker Desktop or the Docker service is running."
  log "2. Check whether ports 3000, 8000, or 5434 are already in use: ./scripts/doctor.sh"
  log "3. Restart the stack: docker compose down && ./start-dataguardian.sh"
  log "4. Reset local data only if needed: docker compose down -v"
  log "5. Inspect logs: docker compose logs backend-go frontend"
}

open_browser_if_requested() {
  if [ ! -t 0 ]; then
    log "Open this URL in your browser: $APP_URL"
    return
  fi

  printf '[DataGuardian] Open DataGuardian in your browser now? [y/N]: '
  read -r answer
  case "$answer" in
    y|Y|yes|YES)
      ;;
    *)
      log "Open this URL when you are ready: $APP_URL"
      return
      ;;
  esac

  if command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$APP_URL" >/dev/null 2>&1 || true
    return
  fi

  if command -v open >/dev/null 2>&1; then
    open "$APP_URL" >/dev/null 2>&1 || true
    return
  fi

  log "Open this URL in your browser: $APP_URL"
}

cd "$ROOT_DIR"

log "Starting DataGuardian"
log "If you downloaded a ZIP, make sure it was extracted first."
log "Keep this launcher in the DataGuardian folder."
log "This helper only runs Docker Compose commands in this folder."
log "It does not install services or change system settings."
log "This may take a few minutes the first time while Docker builds images."

if ! command -v docker >/dev/null 2>&1; then
  log "Docker was not found."
  log "Install Docker Desktop, then run this launcher again:"
  log "https://docs.docker.com/get-docker/"
  log "For beginner help, open START-HERE.txt."
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "Docker is installed, but the Docker daemon is not reachable."
  log "Start Docker Desktop or the Docker service yourself, wait until it is running, then rerun this launcher."
  log "No Docker process was started by this helper."
  log "For beginner help, open START-HERE.txt."
  exit 1
fi

if ! "$ROOT_DIR/scripts/up.sh"; then
  help_recovery
  exit 1
fi

open_browser_if_requested

log "DataGuardian started correctly."
log "App: $APP_URL"
log "Demo users: admin / admin123 or test / test123"
log "Help: START-HERE.txt or README.md"
