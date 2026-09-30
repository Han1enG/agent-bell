#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
OUT_DIR="$ROOT_DIR/dist/release"
APP_DIR="$OUT_DIR/AgentBell.app"
SDKROOT="${SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

rm -rf "$OUT_DIR"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

cd "$ROOT_DIR"
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$APP_DIR/Contents/MacOS/agentbell" .
clang -isysroot "$SDKROOT" -mmacosx-version-min=13.0 -framework Cocoa \
  native/AgentBellNotifier.m -o "$APP_DIR/Contents/MacOS/AgentBellNotifier"
sed "s/>0.1.0</>$VERSION</g" native/Info.plist > "$APP_DIR/Contents/Info.plist"
cp assets/agentbell-icon.png "$APP_DIR/Contents/Resources/AgentBell.png"

ditto -c -k --sequesterRsrc --keepParent "$APP_DIR" \
  "$OUT_DIR/AgentBell-v${VERSION}-macOS.zip"
cp "$APP_DIR/Contents/MacOS/agentbell" "$OUT_DIR/agentbell-v${VERSION}-darwin-arm64"
echo "Built $OUT_DIR"
