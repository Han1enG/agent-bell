# AgentBell v0.4 Release Acceptance

This phase freezes product development. Manual product acceptance cannot be replaced by synthetic state or additional UI automation. This checklist supersedes the release gates in RELEASE_REPORT_V04.md.

Candidate source: `8b4b56ea90b0056b56d2cae2b3f1552e14a0e816`, branch `release/v0.4-acceptance`.

## Verified release checks

- [x] arm64 remote CI (`macos-15`)
- [x] Intel remote CI (`macos-15-intel`), including actual native execution
- [x] tmux E2E on both CI runners
- [x] Signed release bundle smoke on both CI runners
- [x] Downloaded arm64-runner artifacts: checksum, deep/strict signature, executable/version, JSON/read-only status smoke passed locally

[CI run](https://github.com/Han1enG/agent-bell/actions/runs/37273978536)

[arm64 job](https://github.com/Han1enG/agent-bell/actions/runs/37273978536/job/111646825784)

[Intel job](https://github.com/Han1enG/agent-bell/actions/runs/37273978536/job/111646825587)

Synthetic native lifecycle tests also passed on both runners; they do not check any manual item below.

## Required human acceptance — awaiting user results

- [ ] Actual menu icon and Badge
- [ ] NEEDS YOU / WORKING / RECENT visual appearance and ordering
- [ ] Return visible and clickable
- [ ] Pause / Resume / Clear Recent
- [ ] No Dock icon; basic keyboard operation
- [ ] Tabby Claude A WORKING / Claude B NEEDS YOU / Claude C DONE; GoLand Codex D WORKING
- [ ] Badge 1, B needs attention, A/D working, C recent
- [ ] B answers/resumes: NEEDS YOU → WORKING, Badge 1 → 0
- [ ] Actual Menu Bar Return click to correct Tabby session
- [ ] Actual Menu Bar Return click to correct GoLand session
- [ ] Quit App, trigger actual Claude Stop/NeedsInput, system notification appears
- [ ] Click that actual notification: Return-to-Context works

## Accepted non-blocking limitations

- Codex has no reliable automatic NeedsInput upstream hook; permission requests remain observational.
- Physical machine reboot/login launch not tested.
- Local tmux absent; real tmux E2E passed in both CI jobs.
- Developer ID/notarization absent; ad-hoc signing is acceptable for v0.4.0.
- Full cold-launch notification attribution not tested, while the actual quit→notification→Return loop above remains required.
- Old v0.3 notification compatibility may be documented if real cross-version clicking is impractical.
- iTerm2 / WezTerm remain Experimental.

## Manual candidate

Use the downloaded passing CI build at:

`/Users/hanleng1/Code/Toy/agentbell/dist/acceptance/AgentBell.app`

Install its app and owned hooks before manual acceptance:

```sh
/Users/hanleng1/Code/Toy/agentbell/dist/acceptance/AgentBell.app/Contents/MacOS/agentbell install
```

No test-only sessions or UI automation should substitute for these human checks. The command installs the candidate app, refreshes owned hooks and honors existing config.

The downloaded CI archives in `dist/acceptance-ci/` have hashes:

- arm64: `246ccd03406218acf2fdf054d95163db708ac8717db0a4052dbe12559ba267db`
- amd64: `4a327be53461cd3bd228f3fc31ee785db4765303f086a5fbd77a54ec57f5e3ac`

## Publication — only after all required human checks pass

- [ ] Final publication bundle smoke
- [ ] Tag v0.4.0 on accepted source
- [ ] GitHub Release
- [ ] Update Homebrew tap using the exact published archive checksums
- [ ] Real brew upgrade/install/version/doctor validation

The local tap formula still has hashes for the earlier local build. It must be updated against the eventual published packages; rebuilding changes archive hashes. No tag, Release or tap push has occurred.

Release title: **AgentBell v0.4 — Attention Center**

See which coding agents need you.

Keep track of what is still working and what just finished.

Return to the right session in one click.

## Manual acceptance incident: notification arrives, menu remains empty

User screenshot at 2026-10-05 16:01 (Asia/Shanghai): menu icon and sections are visible, but all three sections are empty while a real Codex completion notification appears. This does **not** pass the end-to-end Attention Center gate.

Read-only diagnostics found:

- Running installed App and reachable IPC report 0.4.0 / protocol 1.
- Live IPC snapshot and SQLite sessions are empty.
- PATH resolves agentbell to /opt/homebrew/bin/agentbell, still version 0.3.0.
- Current Claude/Codex hook files point to the 0.4.0 CI candidate CLI.
- The 16:01 notification log follows the legacy dispatch path without an IPC attempt.

Likely cause: an already-running Codex session retains the pre-upgrade hook configuration. This is an inference from the runtime/config mismatch, not yet confirmed by a fresh real Agent session. The next acceptance step is to launch a fresh Claude Code/Codex CLI process in the intended Tabby/GoLand surface and trigger actual work/completion; confirm its events enter the app. No synthetic events were injected and no product code was changed during this diagnosis. Do not restart active agent processes automatically.

## UI feedback revision (2026-10-05)

The user reported no indication of newly completed work, a disappearing menu-bar
bell, oversized footer buttons, and requested a bell/terminal icon redesign.
The user explicitly chose: the number counts NEEDS YOU; unseen completions use a
dot, cleared by opening the popover. Viewing does not clear attention state.

The revised UI uses a filled template bell with a terminal prompt cutout, a blue
vector-rendered app tile, a compact single-line footer action menu, and compact
Return controls. Completion visibility is persisted separately from session
state. The status item has a stable autosave name, removal disabled, eager icon
rendering, and restores its visibility if macOS toggles it off. This is a
mitigation: the original disappearance was not reproduced or proven to be a
crash; the App and core were alive when inspected. macOS overflow remains a
possible cause.

While resident, the App now submits native notifications itself, preserving
return metadata and actions. CLI-only / quit fallback remains the native helper.
Notification permission denial is shown in the popover. Prior logs contained
UNErrorDomain error 1; permission and actual delivery require real verification.

Previous CI run 37273978536 covers the preceding candidate only. The new revision
requires another arm64 / Intel CI run and renewed manual visual acceptance.
No release tag, GitHub Release, or Homebrew publication has occurred.

### Second visual feedback revision

The user rejected the narrow bell, adjacent number, weak content hierarchy,
redundant footer status text, and combined ellipsis/dropdown affordance. The
revised menu glyph has wider shoulders and an integrated numeric corner badge;
unseen Done uses a corner dot. The popover now uses higher-contrast card text,
full-width summaries, measured content height, no repeated generic input summary,
and icon-only notification/gear controls with tooltips and accessibility labels.
The gear menu hides its dropdown indicator.

Run 37308742353 failed on the arm64 runner's Swift compiler type-check timeout in
the previous large SwiftUI body expression. The new layout splits header, list,
diagnostics and footer into independent expressions and removes the manual
height arithmetic. Both architectures must pass the next remote run. Visual
acceptance remains pending on the actual user sessions.

### Arrowless panel and attention lifecycle

The user requested removal of the system popover arrow and clarification of
NEEDS YOU lifetime. The UI now uses a public-API borderless floating NSPanel,
with rounded visual-effect background, screen-bounded positioning, click-outside
and Escape dismissal, and the same Return / notification / settings controls.
This does not depend on private NSPopover arrow-hiding APIs.

NEEDS YOU clears when the same session reports resumed work (moves to WORKING),
completion (moves to RECENT), or a periodic local check confirms its process or
exact context has ended (archived as unknown, not fabricated Done). Opening the
panel, Return, notification pause, and Clear Recent do not resolve attention.
Known live sessions have no elapsed-time expiry. Sessions without reliable
process identity expire after 24 hours without an update. Reconciliation runs at
startup and every minute. Unreachable providers and process permission/timeouts
are inconclusive and preserve attention. RECENT defaults to seven-day retention.

Fixed a lifecycle gap: ERROR attention now participates in the same negative
process/context and missing-identity reconciliation as input/approval waits.
Regression coverage includes incidental activity, resume, Done, closed/error
sessions, and 23h/25h unidentified attention boundaries. Manual panel appearance
and input behavior still require actual user verification.

### Wider panel, unclipped summaries, smaller menu mark

Following real user feedback, panel width increases from 380 to 460 points.
Summary text now keeps its natural wrapped height instead of a three-line limit.
The scroll content resists vertical compression, and a content-height change
explicitly resizes the native panel; frame-change observation alone did not
notify the panel when SwiftUI's intrinsic height changed on a new event.
Long lists still use the bounded scroll viewport. The menu bell shrinks by about
12% (the app tile remains unchanged), keeping count glyphs readable.

Badge rule remains: a number takes precedence and counts NEEDS YOU sessions;
otherwise a dot means at least one unseen completion; neither means no active
attention and no unseen completion. Opening the panel clears completion dots.
A completion arriving while the panel is open is treated as viewed.
