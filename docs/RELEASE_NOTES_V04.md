# AgentBell v0.4 — Attention Center

See every coding agent that needs you.
Return to the right session in one click.

A native menu bar app now shows NEEDS YOU, WORKING and RECENT across Claude Code and Codex. The badge counts sessions requiring input, confirmed approval or error handling. Return reuses Universal Return and does not assume a wait was resolved.

The app owns a Go session engine, versioned private Unix IPC and SQLite v1. State survives restart, stale sessions are reconciled, and hooks fall back to existing notifications when the app is absent. Pause affects notifications only. Additions include status/JSON, bounded logs, app/storage doctor checks and login launch enabled by default.

Upgrade with `brew upgrade agentbell` followed by `agentbell install` after the release and tap update are published. Config and unrelated integrations are preserved. Set `[attention_center] enabled=false` to retain notification-only mode; `launch_at_login=false` disables login launch on the next install.

Codex PermissionRequest remains a pre-routing observed event, never attention. Codex's upstream hooks currently lack an official input-wait notification; real human-wait detection cannot be inferred from permission requests. Claude pre-routing requests likewise do not create attention; a permission_prompt notification confirms a wait.

All session state stays local. No task management, agent orchestration, prompt entry, transcript storage, analytics or cloud service.

Release candidate: real multi-agent UI, native notification clicks, machine restart and live Return acceptance must pass before stable publication. See RELEASE_REPORT_V04.md for verified and outstanding checks.
