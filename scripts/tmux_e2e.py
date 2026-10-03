#!/usr/bin/env python3
"""Real tmux lifecycle checks on an isolated socket and owned fixture PTY.
No production provider creates/kills sessions: this test owns all fixture state.
"""
import fcntl, json, os, pathlib, pty, shutil, struct, subprocess, tempfile, termios, time, statistics

tmux = shutil.which('tmux')
if not tmux:
    raise SystemExit('tmux is required for this E2E test')
root = pathlib.Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix='abtm-', dir='/private/tmp') as tmp:
    sock = tmp + '/custom socket'
    binary = tmp + '/agentbell'
    env = dict(os.environ, GOCACHE='/private/tmp/agentbell-go-cache')
    subprocess.run(['go', 'build', '-o', binary, '.'], cwd=root, env=env, check=True)
    def mux(*args, check=True):
        return subprocess.run([tmux, '-S', sock, *args], text=True, capture_output=True, check=check).stdout.strip()
    clients = []
    fds = []
    def attach(session):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
        def controlling_tty():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)
        client = subprocess.Popen([tmux, '-S', sock, 'attach-session', '-t', session],
            stdin=slave, stdout=slave, stderr=slave, env=dict(env, TERM='xterm-256color'), preexec_fn=controlling_tty)
        os.close(slave); fds.append(master); clients.append(client)
        for _ in range(100):
            if mux('list-clients', '-F', '#{session_id}', check=False): return client
            time.sleep(.02)
        os.set_blocking(master, False)
        raise AssertionError('fixture client did not attach: ' + repr(os.read(master, 8192)))
    def detect(pane):
        server = mux('display-message', '-p', '-t', pane, '#{pid}')
        testenv = dict(env, TMUX=sock + ',' + server + ',0', TMUX_PANE=pane,
            AGENTBELL_SURFACE='', AGENTBELL_CONTEXT_ID='', TERM_PROGRAM='', ITERM_SESSION_ID='', WEZTERM_PANE='')
        result = subprocess.run([binary, 'surface', 'detect'], env=testenv, text=True, capture_output=True, check=True)
        return json.loads(result.stdout)['Layers'][1]
    def action(operation, target, valid=True):
        result = subprocess.run([binary, 'surface', operation, 'tmux', target['ContextID']], env=env, text=True, capture_output=True)
        assert (result.returncode == 0) == valid, (operation, result.stdout, result.stderr, json.loads(target['ContextID']), mux('list-clients', '-F', '#{client_tty} #{client_pid} #{client_created} #{session_id}'), mux('list-panes', '-a', '-F', '#{pane_id} #{window_id} #{session_id} #{pid} #{start_time}'))
        return result
    try:
        pane = mux('-f', '/dev/null', 'new-session', '-d', '-P', '-F', '#{pane_id}', '-s', 'fixture', '-c', tmp)
        client = attach('fixture')
        timings = []
        for _ in range(10):
            begin = time.monotonic(); target = detect(pane); timings.append((time.monotonic() - begin) * 1000)
        print('Detection including subprocess + fixture server lookup: cold=%.1fms warm-median=%.1fms warm-max=%.1fms' % (timings[0], statistics.median(timings[1:]), max(timings[1:])))
        assert target['Capability'] == 'exact_context', target
        action('probe', target); action('focus', target)
        other = mux('split-window', '-d', '-P', '-F', '#{pane_id}', '-t', pane, '-c', tmp)
        mux('select-pane', '-t', other)
        action('probe', target)
        assert mux('display-message', '-p', '-c', mux('list-clients', '-F', '#{client_tty}'), '#{pane_id}') == other, 'probe selected pane'
        mux('resize-pane', '-Z', '-t', other)
        action('focus', target)
        assert mux('display-message', '-p', '-t', pane, '#{window_zoomed_flag}') == '1', 'provider changed zoom state'
        mux('resize-pane', '-Z', '-t', pane)
        assert mux('display-message', '-p', '-c', mux('list-clients', '-F', '#{client_tty}'), '#{pane_id}') == pane
        window = mux('new-window', '-d', '-P', '-F', '#{pane_id}', '-t', 'fixture', '-c', tmp)
        windowtarget = detect(window)
        action('focus', windowtarget)
        mux('kill-window', '-t', window)
        action('focus', windowtarget, False)
        mux('new-session', '-d', '-s', 'other', '-c', tmp)
        action('focus', target)
        mux('kill-pane', '-t', pane)
        replacement = mux('split-window', '-d', '-P', '-F', '#{pane_id}', '-t', other, '-c', tmp)
        assert replacement != pane
        action('focus', target, False)
        live = detect(replacement)
        action('probe', live)
        second_client = attach('fixture')
        for _ in range(100):
            if len(mux('list-clients', '-F', '#{client_pid}').splitlines()) == 2: break
            time.sleep(.02)
        assert detect(replacement)['Capability'] != 'exact_context', 'ambiguous clients claimed exact'
        action('focus', live, False)
        second_client.terminate(); second_client.wait(timeout=3)
        mux('kill-session', '-t', 'fixture')
        action('focus', target, False)
        mux('kill-server')
        # Reuse the same path and numeric pane ID on a new server.
        restarted = mux('-f', '/dev/null', 'new-session', '-d', '-P', '-F', '#{pane_id}', '-s', 'fixture', '-c', tmp)
        attach('fixture')
        assert restarted == pane, (restarted, pane)
        action('focus', target, False)
        print('Real tmux E2E passed: custom socket, attached client, read-only probe, split panes, same cwd, multiple windows/sessions, pane/window/session close, server restart with reused pane ID')
    finally:
        mux('kill-server', check=False)
        for client in clients:
            try: client.wait(timeout=3)
            except subprocess.TimeoutExpired: client.terminate(); client.wait(timeout=3)
        for fd in fds: os.close(fd)
