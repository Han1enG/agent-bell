# AgentBell v0.3 — Universal Return

AgentBell tells you when your coding agent needs you — and takes you back to the right session.

Universal Return understands terminals, editors, and multiplexers as layered work surfaces. v0.3 introduces a provider registry, composite return plans, and exact tmux pane return with attached-client and server-generation checks. `agentbell surfaces`, `surfaces --json`, and doctor Return Stack explain the available return path.

Context Protocol v1 is frozen. Existing v0.2 targets without Layers continue through the single-provider return and fallback path. Breaking protocol changes require a new version.

Exact-return support varies by environment. AgentBell falls back safely instead of guessing. Closed panes, changed instances, ambiguous clients and unavailable permissions must not select a replacement by title or directory.

iTerm2 and WezTerm support is **Experimental**, validated with fixtures only. Their capabilities describe implemented behavior, not completed real GUI validation. GoLand support remains scoped to 2025.3 / build 253.

## Validation and known limitations

Remote macos-15 and macos-15-intel CI passed race/vet, Node/Python tests, the real Java bridge, real isolated tmux lifecycle E2E, native builds, codesign verification and release bundle smoke checks.

Real Tabby+tmux and Terminal+tmux notification-click GUI E2E and stale-notification GUI fallback have **not** been completed because the test environment denied GUI control of these applications. CLI lifecycle tests do not replace those checks. This release is published with these limitations explicitly accepted by the maintainer; it does not claim a fully validated GUI matrix.

Serial warm detection p95 was 27ms and real notification dispatch p95 was 81ms on the tested arm64 machine. Fresh-path cold startup had substantial overhead outside measured Go work; system-level attribution and cold native-helper dispatch remain incomplete.

After upgrading with Homebrew, run `agentbell install`, then `agentbell doctor`. Restart Tabby/GoLand after integration updates and open a new local terminal. Managed integration files are upgraded only when their recorded checksums still match; user modifications are preserved.
