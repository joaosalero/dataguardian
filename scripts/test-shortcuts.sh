#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TEMP_HOME"' EXIT
HOME="$TEMP_HOME" XDG_DATA_HOME="$TEMP_HOME/share with spaces" XDG_DESKTOP_DIR="$TEMP_HOME/Desktop with spaces" "$ROOT_DIR/scripts/desktop-shortcut.sh" install
test -f "$TEMP_HOME/share with spaces/applications/dataguardian.desktop"
HOME="$TEMP_HOME" XDG_DATA_HOME="$TEMP_HOME/share with spaces" XDG_DESKTOP_DIR="$TEMP_HOME/Desktop with spaces" "$ROOT_DIR/scripts/desktop-shortcut.sh" install
HOME="$TEMP_HOME" XDG_DATA_HOME="$TEMP_HOME/share with spaces" XDG_DESKTOP_DIR="$TEMP_HOME/Desktop with spaces" "$ROOT_DIR/scripts/desktop-shortcut.sh" remove
test ! -e "$TEMP_HOME/share with spaces/applications/dataguardian.desktop"
printf 'Shortcut tests passed.\n'
