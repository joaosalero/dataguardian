#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
compose() { if docker compose version >/dev/null 2>&1; then docker compose "$@"; else docker-compose "$@"; fi; }
while true; do
  printf '\nDataGuardian\n1 Start\n2 Stop\n3 Open\n4 Status\n5 Logs\n6 Install shortcut\n7 Remove shortcut\n8 Exit\n> '
  read -r choice
  case "$choice" in
    1) ./start-dataguardian.sh ;;
    2) compose down ;;
    3) command -v xdg-open >/dev/null && xdg-open http://localhost:3000 >/dev/null 2>&1 || command -v open >/dev/null && open http://localhost:3000 ;;
    4) compose ps ;;
    5) compose logs --tail=100 backend-go frontend ;;
    6) if [ "$(uname -s)" = Darwin ]; then ./scripts/desktop-shortcut-macos.sh install; else ./scripts/desktop-shortcut.sh install; fi ;;
    7) if [ "$(uname -s)" = Darwin ]; then ./scripts/desktop-shortcut-macos.sh remove; else ./scripts/desktop-shortcut.sh remove; fi ;;
    8) exit 0 ;;
    *) printf 'Choose a number from 1 to 8.\n' ;;
  esac
done
