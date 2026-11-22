#!/usr/bin/env bash
# Builds TelTun for Linux or macOS (the OS you run it on).
# Needs Go 1.25+, Node.js, the Wails CLI
# (go install github.com/wailsapp/wails/v2/cmd/wails@latest)
# and on Linux: libgtk-3-dev, libwebkit2gtk-4.1-dev (or -4.0-dev), pkg-config.
set -euo pipefail
cd "$(dirname "$0")"

OS=$(go env GOOS)

# Google platform-tools (adb) for this OS are bundled into the binary.
if [ ! -f "assets/adb/$OS/adb" ]; then
  tmp=$(mktemp -d)
  curl -fsSL -o "$tmp/pt.zip" "https://dl.google.com/android/repository/platform-tools-latest-$OS.zip"
  unzip -q "$tmp/pt.zip" -d "$tmp"
  mkdir -p "assets/adb/$OS"
  cp "$tmp/platform-tools/adb" "$tmp/platform-tools/NOTICE.txt" "assets/adb/$OS/"
  rm -rf "$tmp"
fi

for a in arm64 arm amd64; do
  CGO_ENABLED=0 GOOS=linux GOARCH=$a GOARM=7 \
    go build -trimpath -ldflags '-s -w' -o "assets/phone/teltun-$a" ./cmd/teltun-phone
done

# sing-box needs these tags and -checklinkname=0 (see its release/LDFLAGS).
TAGS="with_gvisor badlinkname tfogo_checklinkname0"
if [ "$OS" = linux ] && pkg-config --exists webkit2gtk-4.1; then
  TAGS="$TAGS webkit2_41" # Ubuntu 24.04+ / Debian 13 ship only 4.1
fi
wails build -clean -skipbindings -trimpath -tags "$TAGS" \
  -ldflags '-s -w -checklinkname=0 -X internal/godebug.defaultGODEBUG=multipathtcp=0'
echo "Done: build/bin"
