#!/usr/bin/env bash
set -euo pipefail

BACKEND_URL="${BACKEND_URL:-http://localhost:8000}"
FRONTEND_URL="${FRONTEND_URL:-http://localhost:3000}"
SMOKE_USER="${SMOKE_USER:-admin}"
SMOKE_PASSWORD="${SMOKE_PASSWORD:-admin123}"
SMOKE_URL="${SMOKE_URL:-https://www.iana.org/help/example-domains}"

COOKIE_JAR="$(mktemp)"
SAMPLE_FILE="$(mktemp)"
cleanup() {
  rm -f "$COOKIE_JAR" "$SAMPLE_FILE"
}
trap cleanup EXIT

log() {
  printf '[smoke] %s\n' "$1"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "$1 is required for smoke validation."
    log "Install the missing tool, then retry. For first-run help, open START-HERE.txt."
    exit 1
  fi
}

check_url() {
  local label="$1"
  local url="$2"
  if ! curl -fsS --max-time 5 "$url" >/dev/null 2>&1; then
    log "$label is not reachable at $url"
    log "Start or restart the stack, then retry: ./start-dataguardian.sh"
    log "On Windows, double-click start-dataguardian.bat."
    log "For beginner recovery help, open START-HERE.txt."
    log "For details, inspect logs with: docker compose logs backend-go frontend"
    exit 1
  fi
}

json_value() {
  sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" | head -n 1
}

need curl

log "checking backend health"
check_url "backend health" "$BACKEND_URL/health"

log "checking frontend login page"
check_url "frontend login page" "$FRONTEND_URL/login"

log "checking dashboard route"
check_url "dashboard route" "$FRONTEND_URL/dashboard"

log "logging in as $SMOKE_USER"
if ! curl -fsS --max-time 10 \
  -c "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$SMOKE_USER\",\"password\":\"$SMOKE_PASSWORD\"}" \
  "$BACKEND_URL/auth/login" >/dev/null; then
  log "login failed for smoke user $SMOKE_USER"
  log "Restart the stack, then retry. Default local users are admin / admin123 and test / test123."
  log "For beginner recovery help, open START-HERE.txt."
  exit 1
fi

project_payload="$(curl -fsS --max-time 10 \
  -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Smoke Validation\",\"target\":\"release-smoke\"}" \
  "$BACKEND_URL/projects")"
project_id="$(printf '%s' "$project_payload" | json_value "id")"
if [ -z "$project_id" ]; then
  log "could not read project id from create-project response"
  log "Inspect backend logs with: docker compose logs backend-go"
  log "For beginner recovery help, open START-HERE.txt."
  exit 1
fi

printf 'DataGuardian smoke sample\nmetadata-only validation\n' > "$SAMPLE_FILE"

log "running file analysis"
curl -fsS --max-time 20 \
  -b "$COOKIE_JAR" \
  -F "projectId=$project_id" \
  -F "inputType=FILE" \
  -F "file=@$SAMPLE_FILE;filename=smoke.txt;type=text/plain" \
  "$BACKEND_URL/analyses" >/dev/null

log "running URL analysis for $SMOKE_URL"
curl -fsS --max-time 20 \
  -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"projectId\":$project_id,\"inputType\":\"URL\",\"url\":{\"originalUrl\":\"$SMOKE_URL\"}}" \
  "$BACKEND_URL/analyses" >/dev/null

log "checking analysis history"
curl -fsS --max-time 10 \
  -b "$COOKIE_JAR" \
  "$BACKEND_URL/analyses?page=1&pageSize=5" >/dev/null

log "release smoke validation passed"
