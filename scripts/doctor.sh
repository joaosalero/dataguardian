#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1

DB_CONTAINER="dataguardian_db"
PORTS=(5434 8000 3000)

print_header() {
  printf "\n== %s ==\n" "$1"
}

show_port_owner() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN; then
      return
    fi
  fi

  if command -v ss >/dev/null 2>&1; then
    if ss -ltnp "sport = :$port" 2>/dev/null | grep -q ":$port"; then
      ss -ltnp "sport = :$port" 2>/dev/null || true
      return
    fi
  fi

  if [ "$port" = "8000" ]; then
    pgrep -af "go run ./cmd/server|dataguardian-backend-go" || true
    return
  fi

  if [ "$port" = "3000" ]; then
    pgrep -af "next dev|npm run dev" || true
    return
  fi

  if [ "$port" = "5434" ] && command -v docker >/dev/null 2>&1; then
    docker ps --filter "name=$DB_CONTAINER" --format 'container {{.Names}}: {{.Status}}' || true
    return
  fi

  printf "No process owner details available.\n"
}

service_responds_on_port() {
  local port="$1"

  if ! command -v curl >/dev/null 2>&1; then
    return 1
  fi

  if [ "$port" = "8000" ]; then
    curl -fsS --max-time 2 "http://localhost:8000/health" >/dev/null 2>&1
    return
  fi

  if [ "$port" = "3000" ]; then
    curl -fsS --max-time 2 "http://localhost:3000/login" >/dev/null 2>&1
    return
  fi

  return 1
}

is_port_in_use() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
  fi

  if command -v ss >/dev/null 2>&1; then
    if ss -ltn "sport = :$port" 2>/dev/null | grep -q ":$port"; then
      return 0
    fi
  fi

  if service_responds_on_port "$port"; then
    return 0
  fi

  return 1
}

print_header "DataGuardian doctor"

print_header "Ports"
for port in "${PORTS[@]}"; do
  if is_port_in_use "$port"; then
    printf "Port %s: IN USE\n" "$port"
    show_port_owner "$port"
  else
    printf "Port %s: free\n" "$port"
  fi
done

print_header "Docker"
if ! command -v docker >/dev/null 2>&1; then
  printf "Docker: NOT FOUND\n"
  exit 1
fi

if docker info >/dev/null 2>&1; then
  printf "Docker: running\n"
else
  printf "Docker: NOT RUNNING\n"
  exit 1
fi

print_header "Database container"
if docker ps -a --format '{{.Names}}' | grep -qx "$DB_CONTAINER"; then
  state="$(docker inspect -f '{{.State.Status}}' "$DB_CONTAINER" 2>/dev/null || printf "unknown")"
  printf "Container %s: exists (%s)\n" "$DB_CONTAINER" "$state"
else
  printf "Container %s: not created yet\n" "$DB_CONTAINER"
fi
