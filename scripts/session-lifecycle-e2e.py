#!/usr/bin/env python3
"""Actual native bundle + isolated SQLite/IPC; all hook fixtures are synthetic."""
import argparse, datetime, json, os, pathlib, socket, subprocess, tempfile, time
p=argparse.ArgumentParser();p.add_argument('app',type=pathlib.Path);p.add_argument('--hold',type=int,default=0);a=p.parse_args()
app=a.app.resolve()
with tempfile.TemporaryDirectory(prefix='ab05-',dir='/tmp') as home:
    env=dict(os.environ,HOME=home)
    config=pathlib.Path(home)/'.config/agentbell/config.toml';config.parent.mkdir(parents=True)
    config.write_text('[notifications]\ndone=false\nneeds_input=false\nneeds_approval=false\nerror=false\n')
    binary=app/'Contents/MacOS/agentbell';endpoint=pathlib.Path(home)/'Library/Application Support/AgentBell/agentbell.sock'
    def request(**fields):
        with socket.socket(socket.AF_UNIX) as client:
            client.settimeout(2);client.connect(str(endpoint));client.sendall(json.dumps(dict(version=1,**fields)).encode()+b'\n')
            data=b''
            while not data.endswith(b'\n'):data+=client.recv(65536)
            response=json.loads(data);assert not response.get('error'),response;return response
    def launch():
        process=subprocess.Popen([str(app/'Contents/MacOS/AgentBellApp')],env=env)
        deadline=time.monotonic()+10
        while not endpoint.exists():
            assert process.poll() is None,'native App exited'
            if time.monotonic()>deadline:raise RuntimeError('IPC startup timeout')
            time.sleep(.05)
        return process
    def event(sid,kind):
        request(event=dict(Source='claude',Type=kind,SessionID=sid,AgentFlavor='claude_cli',CWD='/tmp',Project=sid,Timestamp=datetime.datetime.now(datetime.timezone.utc).isoformat()))
    def state():return request(command='status')['state']
    process=launch()
    try:
        event('Waiting for approval','needs_input');assert len(state()['needs_you'])==1
        event('Waiting for approval','session_ended');assert not state()['needs_you'] and len(state()['closed'])==1
        event('Dismiss me','needs_input');request(command='dismiss_session',session_id='claude:Dismiss me');event('Dismiss me','needs_input');assert not state()['needs_you']
        # Rows available for manual visual/accessibility inspection.
        event('GUI waiting','needs_input');event('GUI working','working');event('GUI ready','done')
        print('GUI READY: badge=1, NEEDS YOU / WORKING / READY / Recently Closed; isolated HOME='+home,flush=True)
        if a.hold:time.sleep(a.hold)
        request(command='clear_all');s=state();assert not s['needs_you'] and not s['working'] and not s['recent'] and not s.get('closed')
        process.terminate();process.wait(timeout=5)
        deadline=time.monotonic()+5
        while endpoint.exists() and time.monotonic()<deadline:time.sleep(.05)
        assert not endpoint.exists()
        process=launch();event('Dismiss me','needs_input');assert not state()['needs_you']
        event('Dismiss me','session_started');assert len(state()['working'])==1
        print('PASS: native SessionEnd badge removal, Dismiss, Clear All, SQLite restart, fresh SessionStart',flush=True)
    finally:
        if process.poll() is None:process.terminate();process.wait(timeout=5)
