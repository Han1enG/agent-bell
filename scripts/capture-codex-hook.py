#!/usr/bin/env python3
"""Opt-in fixture tap: stdin only, no decisions, no forwarding, no stdout.

Add as a separate command hook in a fresh Codex session, passing an output
DIRECTORY argument. Existing notification hooks and approval policy stay intact.
Review the redacted file and label the actual observed routing in provenance.
"""
import json
import os
from pathlib import Path
import sys
import uuid

# These documented structural/state fields retain their values. Every other
# string is redacted, including unknown nested tool arguments and future fields.
SEMANTIC = {'hook_event_name', 'tool_name', 'permission_mode', 'type', 'event',
            'event_type', 'approval_status', 'approvals_reviewer', 'behavior',
            'status', 'decision'}


def redact(value, key=''):
    if isinstance(value, dict):
        return {k: redact(v, k) for k, v in value.items()}
    if isinstance(value, list):
        return [redact(v, key) for v in value]
    if isinstance(value, str) and key not in SEMANTIC:
        return '/Users/example/Code/demo' if key == 'cwd' else 'REDACTED'
    return value


def main():
    if len(sys.argv) != 2:
        raise ValueError('capture-codex-hook.py requires an output directory')
    data = sys.stdin.buffer.read(1024 * 1024 + 1)
    if len(data) > 1024 * 1024:
        raise ValueError('hook input exceeds 1 MiB')
    payload = redact(json.loads(data))
    directory = Path(sys.argv[1])
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    if directory.is_symlink() or not directory.is_dir():
        raise ValueError('output directory must be a real directory')
    path = directory / (str(uuid.uuid4()) + '.json')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(payload, output, ensure_ascii=False, indent=2)
        output.write('\n')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Capture failure must never approve/deny/block the operation.
        print('AgentBell fixture capture failed: ' + type(error).__name__, file=sys.stderr)
