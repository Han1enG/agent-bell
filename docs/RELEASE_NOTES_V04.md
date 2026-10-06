# AgentBell v0.4 — Attention Center

See which coding agents need you.

Keep track of what is still working and what just finished.

Return to the right session in one click.

AgentBell now has a native menu bar Attention Center for Claude Code and Codex:

- **NEEDS YOU**: input, confirmed approval or error requires attention.
- **WORKING**: the agent is executing.
- **READY**: the current turn has replied and the conversation can continue.

The numeric badge counts action-required sessions. New replies use an unseen dot;
numbers take priority. Viewing the panel clears the dot. Resident completion
banners are off by default; input/error notifications remain enabled. Session
titles prefer available agent metadata, with project-name fallback. Elapsed times
refresh every second using the original seconds/minutes/hours/days units.
WORKING age measures the current turn, separately from session creation.

Return uses existing Universal Return providers. Exact contexts show **Return**;
Codex Desktop currently shows **Open App**. Individual × and Clear Ready hide
entries without closing or deleting agent conversations. The public JSON schema
still calls READY sessions `recent` for compatibility.

Session state stays local in SQLite. The App owns its embedded Go engine and
private IPC; there is no separately installed daemon. Pause mutes notifications
while tracking continues. **Quit AgentBell stops event delivery and new
notifications until the next App launch.** Crash/unstarted-App fallback retains
basic notifications. Already delivered macOS notifications remain in history.

## Upgrade

```sh
brew upgrade agentbell
agentbell install
agentbell doctor
```

Review and trust AgentBell hooks in Codex Settings → Hooks (CLI: `/hooks`). New
or modified definitions are skipped until trusted. Installation alone does not
prove event delivery; Agent Connections reports actual events received. Restart
agents that were running before initial hook installation, and restart Tabby or
GoLand after integration updates. Existing config is preserved. Login launch is
on by default; `attention_center.launch_at_login=false` disables it on install.

## Known limitations and validation scope

- Codex Desktop supports application-level return; upstream hooks do not reliably
  signal every human-input wait. PermissionRequest alone never creates attention.
- iTerm2 and WezTerm are Experimental. Not every surface has GUI acceptance.
- Bundles use ad-hoc signing, without Developer ID or notarization.
- Physical reboot/login launch is not validated across machines.
- Real four-agent state/badge, every menu-return surface, stale-session closure,
  and crash-fallback notification click acceptance are not all recorded as passed.
  See `docs/RELEASE_ACCEPTANCE_V04.md` for the remaining manual checks.

No cloud service or transcript storage is introduced. Release CI validates both
Apple Silicon and Intel, including signed bundles and native lifecycle tests;
synthetic checks do not substitute for manual UI acceptance.
