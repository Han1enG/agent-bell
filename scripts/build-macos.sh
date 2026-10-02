#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.2.1}"
VERSION="${VERSION#v}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist/release}"
SDKROOT="${SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"
BUILD_DIR="$(mktemp -d "$OUT_DIR/.build.XXXXXX")"
trap 'rm -rf "$BUILD_DIR"' EXIT
cd "$ROOT_DIR"
for ARCH in arm64 amd64; do
  if [[ "$ARCH" == arm64 ]]; then CLANG_ARCH=arm64; else CLANG_ARCH=x86_64; fi
  APP_DIR="$BUILD_DIR/$ARCH/AgentBell.app"
  mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"
  GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$APP_DIR/Contents/MacOS/agentbell" .
  clang -arch "$CLANG_ARCH" -isysroot "$SDKROOT" -mmacosx-version-min=13.0 \
    -framework AppKit -framework UserNotifications -framework Foundation \
    native/AgentBellNotifier.m -o "$APP_DIR/Contents/MacOS/AgentBellNotifier"
  sed "s/>0.2.1</>$VERSION</g" native/Info.plist > "$APP_DIR/Contents/Info.plist"
  cp assets/agentbell-icon.png "$APP_DIR/Contents/Resources/AgentBell.png"
  # Bind the bundle identifier and Info.plist to the notification executable.
  # A linker-only signature cannot identify this app to UserNotifications.
  codesign --force --sign - --identifier com.agentbell.AgentBell.cli "$APP_DIR/Contents/MacOS/agentbell"
  codesign --force --sign - "$APP_DIR"
  codesign --verify --deep --strict "$APP_DIR"
  tar -czf "$OUT_DIR/agentbell_${VERSION}_darwin_${ARCH}.tar.gz" -C "$BUILD_DIR/$ARCH" AgentBell.app
done
(cd "$OUT_DIR" && shasum -a 256 "agentbell_${VERSION}_darwin_arm64.tar.gz" "agentbell_${VERSION}_darwin_amd64.tar.gz" > checksums.txt)
echo "Built arm64 and amd64 release packages in $OUT_DIR"
