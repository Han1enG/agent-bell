#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.5.0-rc.1}"
VERSION="${VERSION#v}"
BUNDLE_VERSION="${VERSION%%-*}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist/release}"
SDKROOT="${SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"
BUILD_DIR="$(mktemp -d "$OUT_DIR/.build.XXXXXX")"
trap 'rm -rf "$BUILD_DIR"' EXIT
cd "$ROOT_DIR"
cat native/AttentionCenter/BellIcon.swift scripts/render-icon.swift > "$BUILD_DIR/render-icon.swift"
swiftc -sdk "$SDKROOT" -module-cache-path "$BUILD_DIR/icon-cache" "$BUILD_DIR/render-icon.swift" -o "$BUILD_DIR/render-icon"
"$BUILD_DIR/render-icon" "$BUILD_DIR/AgentBell.png"
cat native/AttentionCenter/BellIcon.swift native/AttentionCenter/AgentBell.swift > "$BUILD_DIR/AgentBell.swift"
for ARCH in arm64 amd64; do
  if [[ "$ARCH" == arm64 ]]; then CLANG_ARCH=arm64; else CLANG_ARCH=x86_64; fi
  APP_DIR="$BUILD_DIR/$ARCH/AgentBell.app"
  mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"
  GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=1 CC="clang -arch $CLANG_ARCH -isysroot $SDKROOT -mmacosx-version-min=13.0" go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$APP_DIR/Contents/MacOS/agentbell" .
  clang -arch "$CLANG_ARCH" -isysroot "$SDKROOT" -mmacosx-version-min=13.0 \
    -framework AppKit -framework UserNotifications -framework Foundation -framework Carbon \
    native/AgentBellNotifier.m -o "$APP_DIR/Contents/MacOS/AgentBellNotifier"
  swiftc -target "$CLANG_ARCH-apple-macosx13.0" -sdk "$SDKROOT" -module-cache-path "$BUILD_DIR/swift-cache-$ARCH" \
    "$BUILD_DIR/AgentBell.swift" -o "$APP_DIR/Contents/MacOS/AgentBellApp"
  sed -e "s/>0.2.1</>$BUNDLE_VERSION</g" -e 's/>AgentBellNotifier</>AgentBellApp</g' native/Info.plist > "$APP_DIR/Contents/Info.plist"
  /usr/libexec/PlistBuddy -c "Add :AgentBellReleaseVersion string $VERSION" "$APP_DIR/Contents/Info.plist"
  cp "$BUILD_DIR/AgentBell.png" "$APP_DIR/Contents/Resources/AgentBell.png"
  # Bind the bundle identifier and Info.plist to the notification executable.
  # A linker-only signature cannot identify this app to UserNotifications.
  codesign --force --sign - --identifier com.agentbell.AgentBell.cli "$APP_DIR/Contents/MacOS/agentbell"
  codesign --force --sign - "$APP_DIR/Contents/MacOS/AgentBellNotifier"
  codesign --force --sign - "$APP_DIR"
  codesign --verify --deep --strict "$APP_DIR"
  tar -czf "$OUT_DIR/agentbell_${VERSION}_darwin_${ARCH}.tar.gz" -C "$BUILD_DIR/$ARCH" AgentBell.app
done
(cd "$OUT_DIR" && shasum -a 256 "agentbell_${VERSION}_darwin_arm64.tar.gz" "agentbell_${VERSION}_darwin_amd64.tar.gz" > checksums.txt)
echo "Built arm64 and amd64 release packages in $OUT_DIR"
