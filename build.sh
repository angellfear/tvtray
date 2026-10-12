#!/bin/sh
# Builds build/tvtray for Linux (GTK3 + WebKit2GTK 4.1).
# Debian/Ubuntu deps: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
# Fedora:             sudo dnf install gtk3-devel webkit2gtk4.1-devel
set -e
cd "$(dirname "$0")"
VERSION="${1:-0.0.0-dev}"
mkdir -p build
CGO_ENABLED=1 go build -tags gtk3 -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" -o build/tvtray .
ls -l build/tvtray
