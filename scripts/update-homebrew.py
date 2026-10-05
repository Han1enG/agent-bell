#!/usr/bin/env python3
"""Stage a tap formula against the exact local release assets; never publish."""
import argparse, hashlib, pathlib, re
p = argparse.ArgumentParser()
p.add_argument('formula', type=pathlib.Path)
p.add_argument('--assets', type=pathlib.Path, default=pathlib.Path('dist/release'))
p.add_argument('--version', default='0.4.0')
a = p.parse_args()
if not re.fullmatch(r'\d+\.\d+\.\d+', a.version): raise SystemExit('invalid version')
text = a.formula.read_text()
text = re.sub(r'releases/download/v[\d.]+/agentbell_[\d.]+_darwin_', f'releases/download/v{a.version}/agentbell_{a.version}_darwin_', text)
text = re.sub(r'version "[\d.]+"', f'version "{a.version}"', text)
for field, arch in [('arm', 'arm64'), ('intel', 'amd64')]:
    digest = hashlib.sha256((a.assets / f'agentbell_{a.version}_darwin_{arch}.tar.gz').read_bytes()).hexdigest()
    text = re.sub(rf'({field}:\s+")[a-f0-9]{{64}}(")', rf'\g<1>{digest}\2', text)
text = text.replace('Local macOS notifications for Claude Code and Codex CLI', 'Local coding agent Attention Center for macOS')
text = text.replace('#{bin}/agentbell version"', '#{bin}/agentbell version --short"')
text = text.replace('After upgrading: agentbell install, then agentbell doctor --fix', 'After upgrading: agentbell install, then agentbell doctor\n      Attention Center launches at login by default; configure attention_center.launch_at_login=false to opt out.')
a.formula.write_text(text)
print('Staged formula with checksums from exact local assets; publication is separate.')
