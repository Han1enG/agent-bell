# AgentBell v0.3 — Universal Return

AgentBell tells you when your coding agent needs you — and takes you back to the right session.

Universal Return understands terminals, editors, and multiplexers as layered work surfaces. v0.3 introduces a provider registry, composite return plans, and exact tmux pane return with attached-client and server-generation checks. `agentbell surfaces`, `surfaces --json`, and doctor Return Stack explain the available return path.

Context Protocol v1 is frozen. Existing v0.2 targets without Layers continue through the single-provider return and fallback path. Breaking protocol changes require a new version.

Exact-return support varies by environment. AgentBell falls back safely instead of guessing. Closed panes, changed instances, ambiguous clients and unavailable permissions must not select a replacement by title or directory.

iTerm2 and WezTerm support is **Experimental**, validated with fixtures only. Their capabilities describe implemented behavior, not completed real GUI validation. GoLand support remains scoped to 2025.3 / build 253.

This document is the prepared release body. The candidate remains RC until real Tabby+tmux and Terminal+tmux notification-click tests, stale notification GUI fallback, remote arm64/Intel CI, and the release/upgrade gates are complete.
