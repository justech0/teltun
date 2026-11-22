#!/usr/bin/env bash
# Installs TelTun from the tar.gz: binary, menu entry and icon.
# Run with: sudo ./install.sh   (remove with: sudo ./install.sh --uninstall)
set -euo pipefail
cd "$(dirname "$0")"
if [ "${1:-}" = "--uninstall" ]; then
  rm -f /usr/bin/teltun /usr/share/applications/teltun.desktop /usr/share/icons/hicolor/256x256/apps/teltun.png
  echo "TelTun removed."; exit 0
fi
install -Dm755 teltun /usr/bin/teltun
install -Dm644 teltun.desktop /usr/share/applications/teltun.desktop
install -Dm644 teltun.png /usr/share/icons/hicolor/256x256/apps/teltun.png
command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q /usr/share/icons/hicolor || true
echo "TelTun installed. Open it from the app menu or run: teltun"
