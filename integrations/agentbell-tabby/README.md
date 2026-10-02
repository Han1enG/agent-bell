# AgentBell Tabby PoC (experimental)

This is a small Tabby integration validated against a running local Tabby instance. The user has confirmed new local shell inheritance and native notification-click return to the original tab.

## Automatic installation

Run `agentbell install`. When Tabby is detected in the standard macOS application locations, AgentBell installs this bundled integration automatically without prompting, network downloads, npm, or restarting Tabby. The integration loads on Tabby's next start. New local tabs inherit the protocol automatically.

`agentbell install --skip-tabby` remembers an opt-out for installation/updates (it preserves an already-installed plugin). `agentbell install --tabby` enables it again. Repeat install updates managed files; `doctor --fix` repairs an already-enabled managed integration. `uninstall` removes managed, unmodified plugin files and preserves other plugins and local edits.

A hidden ownership/hash manifest allows safe updates and cleanup. Earlier manually installed copies can be adopted only if all bundled files match exactly. Production releases embed these four plugin files and package metadata directly in the Go executable; there is no dependency on the repository being present.

## API evidence

- [AppService](https://docs.tabby.sh/classes/AppService.html): `tabs`, `selectTab`, and tab lifecycle.
- [HostWindowService](https://docs.tabby.sh/classes/HostWindowService.html): `bringToFront`.
- [Official example plugin](https://github.com/Eugeny/tabby-clippy): Angular module plugin registration.
- [Local session implementation](https://github.com/Eugeny/tabby/blob/master/tabby-local/src/session.ts): `TERM_PROGRAM=Tabby` and environment passed before PTY creation.

The installed Tabby bundle also contains `selectTab` and `bringToFront`. A WeakMap assigns a random ID to each terminal leaf (or ordinary top-level tab) for its lifetime; no title/cwd matching is used. A per-window Unix socket accepts only list/focus operations. Focus calls the public APIs inside NgZone. Closed tabs fail focus, allowing AgentBell to return to the app.

The socket is in `~/.cache/agentbell/tabby/<window-uuid>.sock` (0700 directory, 0600 socket). The Go provider accepts only UUID identifiers. No shell/AppleScript/eval is used. A local process running as the user can focus a tab, but cannot execute commands through the bridge.

## Test and manual PoC

```sh
node --test integrations/agentbell-tabby/bridge.test.js integrations/agentbell-tabby/context.test.js
```

Automated tests cover three distinct contexts, same-title tabs, expired focus, per-tab environment isolation, and exclusion of already-running/recovered/SSH sessions. The plugin has loaded in the installed Tabby, and external AgentBell focus requests have switched three real tabs with activeTab confirmed through the bridge status API. The user also confirmed `Capability=exact_context` in a newly opened local Tab and notification-click return to that Tab.

For a manual validation, load this folder as `tabby-agentbell` in a development plugins directory using Tabby's documented `TABBY_PLUGINS` mechanism, or the Plugins directory in Settings. Restarting/reloading Tabby can interrupt sessions; do this when safe. The plugin is now installed in the user's Plugins/node_modules/tabby-agentbell directory and Tabby was restarted with explicit user authorization.

1. Open three tabs A/B/C in the test window.
2. Find the window UUID in the socket filename under `~/.cache/agentbell/tabby`.
3. Run `agentbell surface list tabby <window-uuid>`.
4. From another app, run `agentbell surface focus tabby <B-context-id>`.
5. Verify Tabby becomes foreground and B is selected; repeat for A/C.
6. New local tabs receive these values automatically through the tab-open lifecycle. Recovered PTYs and existing agents keep their old environment. For an existing local tab, before starting a new agent, export:

```sh
export AGENTBELL_SURFACE=tabby
export AGENTBELL_CONTEXT_ID='<B-context-id>'
```

7. Send a native notification from B, move to Chrome, click “返回会话”.
8. Close B and repeat from its old notification; it should activate Tabby via app fallback.

## Status and limitations

API feasibility, real Tab focus, new-shell environment inheritance, and native notification-click return: **verified** on this machine. Automatic environment injection for new local Tab profile options is implemented and shell inheritance is verified. The bridge maps leaf contexts and uses the public split focus API; split-pane E2E is not verified. Recovered PTYs and existing shell/agent processes cannot acquire new environment variables automatically. Contexts expire across restarts. SSH injection and tmux are not implemented.

No URL-scheme, Accessibility, or PID/TTY workaround was introduced because the public plugin API provides a plausible focus path. A bridge failure does not affect notifications.

## New-tab environment injection

The integration listens to AppService.tabOpened$ and SplitTabComponent.tabAdded$. It clones local per-tab profile options before PTY creation, adding only AGENTBELL_SURFACE and AGENTBELL_CONTEXT_ID. It does not write commands into the terminal, patch Tabby methods, edit shared profiles, or modify SSH sessions. Recovered PTYs are skipped because their environment already exists.

## Local E2E result — 2026-10-02

After installing and restarting Tabby, a user-created local Tab returned a nonempty protocol ContextID and `Capability=exact_context` from `agentbell surface detect`. The user then reported that the notification returned to that Tab. This validates the complete local Tab → shell environment → AgentBell target → native notification click → original Tab flow. Restored PTYs, SSH, split-pane and App-missing project fallback UI are separate limitations/tests.
