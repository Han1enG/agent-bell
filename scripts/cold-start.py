#!/usr/bin/env python3
"""Serial, real tmux detection benchmark. Build before running; never concurrently.
Fresh executable paths are cold-ish, not an OS cache purge or fresh machine.
Go timings begin after runtime initialization; wall minus total is un-attributed
runtime/OS launch/wait overhead and is not proof of a Gatekeeper delay.
"""
import fcntl, json, os, pathlib, pty, re, shutil, statistics, struct, subprocess, sys, tempfile, termios, time, threading, select
binary = pathlib.Path(sys.argv[1]).resolve()
output = pathlib.Path(sys.argv[2])
notification_mode = len(sys.argv) > 3 and sys.argv[3] == "--notify"
tmux = shutil.which('tmux')
assert tmux and binary.is_file()
with tempfile.TemporaryDirectory(prefix='abperf-', dir='/private/tmp') as tmp:
    sock = tmp + '/socket'
    def mux(*args, check=True):
        return subprocess.run([tmux, '-S', sock, *args], capture_output=True, text=True, check=check).stdout.strip()
    client = None
    master = None
    try:
        pane = mux('-f', '/dev/null', 'new-session', '-d', '-P', '-F', '#{pane_id}', '-s', 'perf', '-c', tmp)
        server = mux('display-message', '-p', '-t', pane, '#{pid}')
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH',24,80,0,0))
        def controlling_tty():
            os.setsid(); fcntl.ioctl(0, termios.TIOCSCTTY, 0)
        client = subprocess.Popen([tmux, '-S', sock, 'attach-session', '-t', 'perf'],stdin=slave,stdout=slave,stderr=slave,env=dict(os.environ,TERM='xterm-256color'),preexec_fn=controlling_tty)
        os.close(slave)
        def drain():
            try:
                while client.poll() is None:
                    if select.select([master], [], [], .1)[0]:
                        if not os.read(master, 65536): break
            except OSError: pass
        threading.Thread(target=drain, daemon=True).start()
        for _ in range(100):
            if mux('list-clients', '-F', '#{client_pid}', check=False): break
            time.sleep(.02)
        else: raise RuntimeError('client not attached')
        env = dict(os.environ, TMUX=sock+','+server+',0',TMUX_PANE=pane,AGENTBELL_DEBUG_TIMING='1',TERM_PROGRAM='',AGENTBELL_SURFACE='',AGENTBELL_CONTEXT_ID='',ITERM_SESSION_ID='',WEZTERM_PANE='')
        if notification_mode:
            env.update(HOME=tmp, AGENTBELL_NOTIFIER=str(binary.parent/'AgentBellNotifier'))
        records=[]
        def sample(path, group):
            start=time.perf_counter()
            command = [str(path), 'notify', '--source', 'codex'] if notification_mode else [str(path), 'surface', 'detect']
            payload = json.dumps(dict(type='Stop', session_id='agentbell-rc-perf-'+str(len(records)), cwd=tmp, project='AgentBell RC timing', message='Release candidate performance check')) if notification_mode else None
            proc=subprocess.run(command,input=payload,capture_output=True,text=True,env=env,check=True)
            wall=(time.perf_counter()-start)*1000
            if not notification_mode:
                target=json.loads(proc.stdout)
                assert target['Layers'][1]['Capability']=='exact_context'
            else:
                assert 'notified codex done' in proc.stdout
            stages={k:float(v) for k,v in re.findall(r'stage=(\w+) duration_ms=([0-9.]+)',proc.stderr)}
            records.append(dict(group=group,wall_ms=wall,stages=stages,external_unattributed_ms=wall-stages['total']))
        sample(binary,'first-ever-path')
        for i in range(0 if notification_mode else 20):
            path=pathlib.Path(tmp)/('cold-'+str(i));shutil.copy2(binary,path)
            sample(path,'cold-ish-fresh-path')
        for _ in range(20): sample(binary,'warm')
        def summarize(group):
            rows=[r for r in records if r['group']==group]
            def stats(values):
                values=sorted(values);return dict(p50=statistics.median(values),p95=values[max(0,__import__('math').ceil(.95*len(values))-1)],max=max(values))
            return dict(count=len(rows),first_run_ms=rows[0]['wall_ms'],wall_ms=stats([r['wall_ms'] for r in rows]),app_total_ms=stats([r['stages']['total'] for r in rows]),tmux_detect_ms=stats([r['stages']['tmux_detect'] for r in rows]),external_unattributed_ms=stats([r['external_unattributed_ms'] for r in rows]))
        result=dict(method=__doc__,command='real native notify with real single attached tmux client; isolated home config/cache' if notification_mode else 'surface detect with real single attached tmux client; notification dispatch not included',summary={g:summarize(g) for g in (('first-ever-path','warm') if notification_mode else ('first-ever-path','cold-ish-fresh-path','warm'))},records=records)
        output.write_text(json.dumps(result,indent=2)+'\n')
        print(json.dumps(result['summary'],indent=2))
    finally:
        mux('kill-server',check=False)
        if client:
            try:client.wait(timeout=3)
            except subprocess.TimeoutExpired:client.kill();client.wait(timeout=3)
        if master is not None:os.close(master)
