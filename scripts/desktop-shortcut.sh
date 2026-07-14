#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-install}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
APP_FILE="$APP_DIR/dataguardian.desktop"
DESKTOP_DIR="${XDG_DESKTOP_DIR:-$HOME/Desktop}"

case "$ACTION" in
  remove)
    rm -f "$APP_FILE"
    [ -d "$DESKTOP_DIR" ] && rm -f "$DESKTOP_DIR/DataGuardian.desktop"
    printf 'DataGuardian shortcuts removed.\n'
    ;;
  install)
    command -v docker >/dev/null 2>&1 || { printf 'Docker is required before installing the shortcut.\n' >&2; exit 1; }
    mkdir -p "$APP_DIR"
    cat >"$APP_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=DataGuardian
Comment=Start DataGuardian with Docker Compose
Exec=bash "$ROOT_DIR/start-dataguardian.sh"
Terminal=true
Categories=Utility;Security;
EOF
    chmod 700 "$APP_FILE"
    if [ -d "$DESKTOP_DIR" ]; then
      cp "$APP_FILE" "$DESKTOP_DIR/DataGuardian.desktop"
      chmod 700 "$DESKTOP_DIR/DataGuardian.desktop"
    fi
    printf 'DataGuardian shortcut installed for the current user.\n'
    ;;
  *) printf 'Usage: %s [install|remove]\n' "$0" >&2; exit 2 ;;
esac
