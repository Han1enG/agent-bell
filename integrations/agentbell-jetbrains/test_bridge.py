#!/usr/bin/env python3
"""Exercise the shipped Java IPC implementation with the real Go CLI."""
from pathlib import Path
import json, os, socket, stat, subprocess, sys, tempfile

root = Path(__file__).resolve().parent
ide = Path(sys.argv[1] if len(sys.argv) > 1 else str(Path.home() / 'Applications/GoLand.app')) / 'Contents'
java_home = Path(os.environ.get('AGENTBELL_JAVA_HOME', str(ide / 'jbr/Contents/Home')))
gson = Path(os.environ.get('AGENTBELL_GSON_JAR', str(ide / 'lib/module-intellij.libraries.gson.jar')))
with tempfile.TemporaryDirectory(prefix='abjb-', dir='/private/tmp') as temp:
    home = Path(temp)
    classes = home / 'classes'
    classes.mkdir()
    cp = str(gson)
    subprocess.run([str(java_home / 'bin/javac'), '-proc:none', '-cp', cp, '-d', str(classes),
                    str(root / 'src/com/agentbell/Bridge.java'), str(root / 'tests/BridgeProbe.java')], check=True)
    binary = home / 'agentbell'
    env = dict(os.environ, GOCACHE='/private/tmp/agentbell-go-cache')
    subprocess.run(['go', 'build', '-o', str(binary), '.'], cwd=root.parent.parent, env=env, check=True)
    probe = subprocess.Popen([str(java_home / 'bin/java'), '-Duser.home=' + str(home),
                             '-cp', cp + ':' + str(classes), 'BridgeProbe'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    try:
        instance = probe.stdout.readline().strip()
        assert instance, 'bridge failed to start'
        path = home / '.cache/agentbell/jetbrains' / (instance + '.sock')
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
        assert stat.S_IMODE(path.parent.stat().st_mode) == 0o700
        env['HOME'] = str(home)
        def run(*args, check=True):
            return subprocess.run([str(binary), *args], env=env, text=True, capture_output=True, check=check)
        contexts = json.loads(run('surface', 'list', 'jetbrains', instance).stdout)
        assert len(contexts) == 2 and contexts[0]['ContextID'] != contexts[1]['ContextID']
        run('surface', 'focus', 'jetbrains', contexts[1]['ContextID'])
        after = json.loads(run('surface', 'list', 'jetbrains', instance).stdout)
        assert not after[0]['Selected'] and after[1]['Selected']
        assert run('surface', 'focus', 'jetbrains', instance + ':00000000-0000-4000-8000-000000000003', check=False).returncode != 0
        env['AGENTBELL_SURFACE'] = 'jetbrains'
        env['AGENTBELL_CONTEXT_ID'] = contexts[1]['ContextID']
        detected = json.loads(run('surface', 'detect').stdout)
        assert detected['Capability'] == 'exact_context'
        for request in [b'{"operation":"execute","context":"shell input"}\n', b'x' * 8192 + b'\n']:
            with socket.socket(socket.AF_UNIX) as client:
                client.settimeout(4)
                client.connect(str(path))
                client.sendall(request)
                assert not json.loads(client.recv(1024))['ok']
        # Bad requests must not kill the bridge.
        assert len(json.loads(run('surface', 'list', 'jetbrains', instance).stdout)) == 2
        print('Java↔Go bridge passed: permissions, unique IDs, detection, focus, expiry, bounded requests')
    finally:
        probe.stdin.write('\n'); probe.stdin.flush()
        probe.wait(timeout=5)
    assert not path.exists(), 'bridge socket was not cleaned up'
