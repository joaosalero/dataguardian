#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-install}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$HOME/Applications/DataGuardian.app"

if [ "$(uname -s)" != "Darwin" ]; then
  printf 'This helper is intended for macOS.\n' >&2
  exit 1
fi
if [ "$ACTION" = "remove" ]; then
  rm -rf "$APP_DIR"
  printf 'DataGuardian app shortcut removed.\n'
  exit 0
fi
[ "$ACTION" = "install" ] || { printf 'Usage: %s [install|remove]\n' "$0" >&2; exit 2; }
command -v osacompile >/dev/null 2>&1 || { printf 'osacompile is unavailable.\n' >&2; exit 1; }
mkdir -p "$HOME/Applications"
rm -rf "$APP_DIR"
launcher="${ROOT_DIR//\\/\\\\}/start-dataguardian.sh"
launcher="${launcher//\"/\\\"}"
osacompile -o "$APP_DIR" -e "tell application \"Terminal\" to do script \"bash \\\"$launcher\\\"\""
printf 'DataGuardian app shortcut installed in ~/Applications.\n'
