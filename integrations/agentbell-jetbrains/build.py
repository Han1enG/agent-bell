#!/usr/bin/env python3
"""Build against the installed 2025.3 IDE SDK, without Gradle/network downloads."""
from pathlib import Path
import subprocess, sys, tempfile, zipfile

root = Path(__file__).resolve().parent
ide = Path(sys.argv[1] if len(sys.argv) > 1 else str(Path.home() / 'Applications/GoLand.app')) / 'Contents'
compiler = ide / 'jbr/Contents/Home/bin/javac'
jars = sorted((ide / 'lib').rglob('*.jar')) + sorted((ide / 'plugins/terminal/lib').rglob('*.jar'))
with tempfile.TemporaryDirectory(prefix='agentbell-java-') as temp:
    subprocess.run([str(compiler), '-proc:none', '--release', '21', '-classpath', ':'.join(map(str, jars)),
                    '-d', temp, *map(str, sorted((root / 'src').rglob('*.java')))], check=True)
    output = root / 'agentbell.jar'
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for base in [Path(temp), root / 'resources']:
            for path in sorted(base.rglob('*')):
                if path.is_file():
                    info = zipfile.ZipInfo(str(path.relative_to(base)), (2026, 10, 2, 0, 0, 0))
                    info.compress_type = zipfile.ZIP_DEFLATED
                    archive.writestr(info, path.read_bytes())
    print(output)
