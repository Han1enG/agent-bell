#!/usr/bin/env python3
"""Launch the actual bundle with synthetic hook events and an isolated Go store.
No real prompts or agent sessions are changed. Keep it open for visual QA, then
Ctrl-C to verify process exit/restore and remove the test directory.
"""
import argparse, datetime, json, os, pathlib, socket, statistics, subprocess, tempfile, time
p = argparse.ArgumentParser()
p.add_argument('app', type=pathlib.Path)
p.add_argument('--hold', type=int, default=180)
a = p.parse_args()
app = a.app.resolve()
with tempfile.TemporaryDirectory(prefix='abui-', dir='/tmp') as home:
    env = dict(os.environ, HOME=home)
    config = pathlib.Path(home) / '.config/agentbell/config.toml'
    config.parent.mkdir(parents=True)
    config.write_text('[notifications]\ndone=false\nneeds_input=false\nneeds_approval=false\nerror=false\n')
    binary = app / 'Contents/MacOS/agentbell'
    process = subprocess.Popen([str(app / 'Contents/MacOS/AgentBellApp')], env=env)
    try:
        endpoint = pathlib.Path(home) / 'Library/Application Support/AgentBell/agentbell.sock'
        deadline = time.monotonic() + 10
        while not endpoint.exists():
            if process.poll() is not None: raise RuntimeError('App exited')
            if time.monotonic() > deadline: raise RuntimeError('App IPC did not start')
            time.sleep(.05)
        def hook(source, sid, kind, project):
            event = dict(session_id=sid, hook_event_name=kind, project=project, cwd='/tmp')
            subprocess.run([str(binary), 'notify', '--source', source], input=json.dumps(event), text=True, env=env, check=True, capture_output=True)
        hook('claude', 'A', 'UserPromptSubmit', 'agent-teams')
        hook('codex', 'B', 'needs_input', 'coordinator')
        hook('claude', 'C', 'Stop', 'api2mcp')
        hook('codex', 'D', 'UserPromptSubmit', 'maestro')
        def status():
            return json.loads(subprocess.check_output([str(binary), 'status', '--json'], env=env))
        state = status()
        assert len(state['needs_you']) == 1 and len(state['working']) == 2 and len(state['recent']) == 1, state
        samples = []
        for _ in range(100):
            started = time.monotonic()
            with socket.socket(socket.AF_UNIX) as client:
                client.settimeout(.04)
                client.connect(str(endpoint))
                client.sendall(json.dumps(dict(version=1, event=dict(Source='claude', Type='tool_activity', SessionID='A', Timestamp=datetime.datetime.now(datetime.timezone.utc).isoformat()))).encode() + b'\n')
                response = b''
                while not response.endswith(b'\n'): response += client.recv(4096)
                assert not json.loads(response).get('error')
            samples.append((time.monotonic() - started) * 1000)
        print('IPC event+ack milliseconds: median=%.3f p95=%.3f max=%.3f' % (statistics.median(samples), sorted(samples)[94], max(samples)), flush=True)
        print('READY: native menu badge=1; A/D working, B needs input, C recent. Isolated HOME=' + home, flush=True)
        try: time.sleep(a.hold)
        except KeyboardInterrupt: pass
        subprocess.run([str(binary), 'attention-control', 'pause'], env=env, check=True)
        hook('codex', 'B', 'UserPromptSubmit', 'coordinator')
        state = status()
        if len(state['needs_you']) != 0:
            print(json.dumps(state, indent=2), flush=True)
            print((pathlib.Path(home) / 'Library/Logs/AgentBell/agentbell.log').read_text(), flush=True)
        assert len(state['needs_you']) == 0
        process.terminate(); process.wait(timeout=5)
        deadline = time.monotonic() + 5
        while endpoint.exists() and time.monotonic() < deadline: time.sleep(.05)
        assert not endpoint.exists(), 'app-owned core survived parent exit'
        state = status()
        assert len(state['working']) == 3 and len(state['recent']) == 1
        assert state['paused']
        process = subprocess.Popen([str(app / 'Contents/MacOS/AgentBellApp')], env=env)
        deadline = time.monotonic() + 10
        while not endpoint.exists():
            if time.monotonic() > deadline: raise RuntimeError('App restart IPC timeout')
            time.sleep(.05)
        state = status()
        assert state['paused'] and len(state['working']) == 3 and len(state['recent']) == 1
        subprocess.run([str(binary), 'attention-control', 'clear_recent'], env=env, check=True)
        assert len(status()['recent']) == 0 and len(status()['working']) == 3
        subprocess.run([str(binary), 'attention-control', 'resume'], env=env, check=True)
        assert not status()['paused']
        process.terminate(); process.wait(timeout=5)
        deadline = time.monotonic() + 5
        while endpoint.exists() and time.monotonic() < deadline: time.sleep(.05)
        assert not endpoint.exists()
        config.write_text('[attention_center]\nenabled=false\n')
        process = subprocess.Popen([str(app / 'Contents/MacOS/AgentBellApp')], env=env)
        time.sleep(1)
        assert not endpoint.exists(), 'CLI-only mode started tracking'
        process.terminate(); process.wait(timeout=5)
        print('PASS: badge clears while paused; app/core exit; offline and App restart restore; pause persists; clear preserves active; CLI-only starts no host.', flush=True)
    finally:
        if process.poll() is None: process.terminate(); process.wait(timeout=5)
