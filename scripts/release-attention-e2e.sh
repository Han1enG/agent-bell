#!/bin/bash
set -euo pipefail
ASSETS="${1:?release asset directory required}"
VERSION="${2:-0.5.0-rc.5}"
case "$(uname -m)" in arm64) ARCH=arm64;; x86_64) ARCH=amd64;; *) exit 1;; esac
WORK="$(mktemp -d /tmp/ab-release-e2e.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
tar -xzf "$ASSETS/agentbell_${VERSION}_darwin_${ARCH}.tar.gz" -C "$WORK"
python3 "$(dirname "$0")/attention-ui-e2e.py" "$WORK/AgentBell.app" --hold 0

python3 "$(dirname "$0")/session-lifecycle-e2e.py" "$WORK/AgentBell.app"
