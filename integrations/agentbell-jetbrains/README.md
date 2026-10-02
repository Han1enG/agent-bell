# AgentBell JetBrains bridge

Local GoLand 2025.3 (build 253) integration for Classic and Reworked terminal
engines. The plugin gives each new local session an opaque UUID via
`LocalTerminalCustomizer`, retaining the existing shell command and environment.
It retrieves that ID from Classic `ShellStartupOptions` or Reworked
`TerminalView.startupOptionsDeferred`, then selects the matching Content and
activates the Terminal tool window on the IDE event thread.

Only `list` and `focus` are accepted over a per-process Unix socket under
`~/.cache/agentbell/jetbrains`. Directory permissions are 0700; socket 0600.
Requests are bounded and timed out. The plugin does not read terminal output,
send terminal input, create sessions, or identify tabs by title or directory.
Existing sessions without injected IDs, SSH, tmux panes, remote IDEs and other
IDE build lines do not have verified exact context support.

Build against an installed GoLand SDK (JDK supplied by the IDE):

```sh
/usr/bin/python3 integrations/agentbell-jetbrains/build.py ~/Applications/GoLand.app
```

The deterministic `agentbell.jar` is embedded into the Go binary. `agentbell
install` installs it into the detected IDE's plugin directory without network
downloads. `--skip-goland` remembers an opt-out; `--goland` enables it. Managed
hashes protect existing foreign or edited files. Uninstall removes only verified
managed files. Installation does not restart the IDE; restart GoLand and create
a new local terminal tab before testing.

API reference: https://plugins.jetbrains.com/docs/intellij/embedded-terminal.html

`Probe` uses `list` and never activates a project window or terminal tab.
Focus rechecks content validity, the owning project frame, and its request
deadline before changing selection. Missing/disposed content and unavailable
frames fall back through Core with a standard reason. A successful SDK build
and Java IPC test do not replace Classic/Reworked GUI acceptance; that complete
matrix is still pending for this development version.
