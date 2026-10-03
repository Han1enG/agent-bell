# Universal Return v0.3 implementation and validation

This is an implementation candidate. Full GUI release acceptance is pending;
unit/IPC/real tmux server checks do not establish notification-to-GUI E2E success.

## Architecture

`builtin.Registry` owns trusted registration. Ordered outer detection preserves
Tabby/Terminal/GoLand behavior; tmux detection is additive. Generic remains last.
`ReturnTarget` preserves old payload fields, adds opaque metadata and two layers.
`BuildPlan` describes app → outer → inner; click execution reconstructs the fixed
plan and probes contexts independently. Legacy provider APIs are adapted through
standard `ProbeResult` / `ReturnResult`, retaining source compatibility.

Composite return requires one attached tmux client and outer TTY binding. The
Tabby bridge exposes the local PTY PID only; Terminal uses session-process TTY;
iTerm uses session identity and TTY. WezTerm uses the official CLI list `tty_name` field for this binding. Missing
TTY metadata or mismatching inherited pane IDs fall back to app.
Basic-only outer apps also fall back until their visible client can be proven.

## Official API research

- [tmux maintained manual](https://github.com/tmux/tmux/blob/master/tmux.1): stable pane/window/session IDs, server `pid`/`start_time`, `client_tty`/`client_pid`/`client_created`, custom `-S` socket, select-window/select-pane. Never find panes by cwd.
- [iTerm2 scripting reference](https://iterm2.com/documentation-scripting.html): window/tab/session objects, session `unique id`, `tty`, and selection. Tab indexes are transient observations; they are never saved as identities. Session resolution happens by UUID and TTY in the live hierarchy. The
  [official scripting dictionary](https://gitlab.com/gnachman/iterm2/-/raw/master/iTerm2.sdef) maps session `unique ID` to the native `guid`.
- [WezTerm instance targeting](https://wezterm.org/cli/cli/index.html), [list](https://wezterm.org/cli/cli/list.html), [activate-pane](https://wezterm.org/cli/cli/activate-pane.html), [CLI source](https://github.com/wezterm/wezterm/blob/main/wezterm/src/cli/mod.rs): explicit socket, `--no-auto-start`, list JSON and pane activation. [Pane source](https://github.com/wezterm/wezterm/blob/main/mux/src/pane.rs) allocates monotonic IDs within a process; socket generation and GUI PID/start time protect restart reuse. Exact WezTerm
return requires a GUI-owned `gui-sock-PID`; the native helper activates that
specific PID before CLI pane selection. Standalone detached mux sockets fall back
to app, rather than claiming visible GUI exact return.
- [Ghostty official AppleScript API](https://ghostty.org/docs/features/applescript): introduced in 1.3.0; window/tab/terminal IDs and `focus` exist. No enhanced Ghostty provider is included in this candidate. Stable identity injection, restart behavior and real GUI validation remain future work; basic app return remains available through ancestry.

## Validation boundaries

Automated Go fixtures cover stale pane/window/session, same-cwd distinct panes,
multiple windows, custom socket, server generation/PID reuse, detached/replaced
clients, ambiguous clients, per-instance WezTerm selection, iTerm app restart,
read-only probes and provider panic fallback. CLI JSON discovery has schema tests.
README provider capability rows are checked against runtime declarations.

`python3 scripts/tmux_e2e.py` creates a private custom server and owned PTY to
exercise real tmux lifecycle. Test fixtures may create/kill their own sessions;
production Return Providers only probe/select. CI installs tmux and runs this
script. Node tests cover Tabby context lifecycle and read-only PTY PID metadata.
Java bridge tests run locally against the installed GoLand SDK; CI compiles and
exercises the real Bridge.java with Temurin 21 and pinned Gson 2.11.0.

## Required GUI acceptance (pending)

1. Tabby + tmux: two tabs, two panes with the same cwd; Chrome foreground;
   Codex/Claude notification click must restore the original tab and pane.
2. Terminal + tmux: select a different window/tab; click restores original
   session TTY and tmux pane; denied Automation falls back to app.
3. iTerm2: windows/tabs/splits with same cwd; click each recorded UUID;
   close session, restart app, verify old notifications never select replacements.
4. WezTerm: GUI windows/tabs/splits, multiple GUI instances, closed panes and GUI
   restart; saved socket must never drift to another instance. Detached standalone
   mux contexts must not be represented as visible exact GUI return.
5. Old notification after pane closure: app fallback, no alternative pane selection.

No GUI result above is marked passed just because fixture tests pass. iTerm2 and
WezTerm were absent on the implementation host. Ghostty is deferred. Remote,
nested tmux, multiple attached clients and unsupported outer binding are not
exact-return claims in v0.3. No menu bar, history, daemon, Web UI or v0.4 work.

## Upgrade

Existing `brew upgrade agentbell` then `agentbell install` upgrades the bundled
Tabby integration (v0.3.0) through the existing checksummed managed-file mechanism.
Locally modified integrations remain protected. `doctor --fix` only upgrades
previously enabled managed integrations. No iTerm/WezTerm startup scripts are
installed or required. Publish a release and update tap checksums only after the
outstanding GUI acceptance; this change does not publish a release.
