#!/bin/bash
set -euo pipefail
ASSETS="${1:?release asset directory required}"
VERSION="${2:-0.3.0}"
(cd "$ASSETS" && shasum -a 256 -c checksums.txt)
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
for ARCH in arm64 amd64; do
  mkdir -p "$WORK/$ARCH"
  tar -xzf "$ASSETS/agentbell_${VERSION}_darwin_${ARCH}.tar.gz" -C "$WORK/$ARCH"
  codesign --verify --deep --strict "$WORK/$ARCH/AgentBell.app"
done
case "$(uname -m)" in arm64) HOST_ARCH=arm64;; x86_64) HOST_ARCH=amd64;; *) exit 1;; esac
BIN="$WORK/$HOST_ARCH/AgentBell.app/Contents/MacOS/agentbell"
test "$("$BIN" version)" = "$VERSION"
"$BIN" surfaces --json > "$WORK/surfaces.json"
python3 - "$WORK/surfaces.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value['schema_version'] == 1
providers = {p['name']: p for p in value['providers']}
assert len(providers) == 7
for name in ('iterm', 'wezterm'):
    assert providers[name]['maturity'] == 'experimental'
assert providers['tmux']['maturity'] == 'stable'
PY
"$BIN" help > /dev/null
echo 'Release bundle smoke passed'
