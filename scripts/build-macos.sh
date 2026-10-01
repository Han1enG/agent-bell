#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.2.0}"
VERSION="${VERSION#v}"
OUT_DIR="$ROOT_DIR/dist/release"
SDKROOT="${SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
for ARCH in arm64 amd64; do
  if [[ "$ARCH" == arm64 ]]; then CLANG_ARCH=arm64; else CLANG_ARCH=x86_64; fi
  APP_DIR="$OUT_DIR/AgentBell.app"
  rm -rf "$APP_DIR"
  mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"
  GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$APP_DIR/Contents/MacOS/agentbell" .
  clang -arch "$CLANG_ARCH" -isysroot "$SDKROOT" -mmacosx-version-min=13.0 \
    -framework AppKit -framework UserNotifications -framework Foundation \
    native/AgentBellNotifier.m -o "$APP_DIR/Contents/MacOS/AgentBellNotifier"
  sed "s/>0.2.0</>$VERSION</g" native/Info.plist > "$APP_DIR/Contents/Info.plist"
  cp assets/agentbell-icon.png "$APP_DIR/Contents/Resources/AgentBell.png"
  tar -czf "$OUT_DIR/agentbell_${VERSION}_darwin_${ARCH}.tar.gz" -C "$OUT_DIR" AgentBell.app
done
(cd "$OUT_DIR" && shasum -a 256 "agentbell_${VERSION}_darwin_arm64.tar.gz" "agentbell_${VERSION}_darwin_amd64.tar.gz" > checksums.txt)
echo "Built arm64 and amd64 release packages in $OUT_DIR"
