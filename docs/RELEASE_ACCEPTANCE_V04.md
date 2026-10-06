# AgentBell v0.4 Release Acceptance

This phase freezes product development. Manual product acceptance cannot be replaced by synthetic state or additional UI automation. This checklist supersedes the release gates in RELEASE_REPORT_V04.md.

Released source: `be3ff09dddcc410397a3b51a6b338a8254dc608d`, tag `v0.4.0`. Earlier candidate records below are historical.

## Final candidate gates — results and remaining manual checks

Earlier green CI runs below are historical evidence, not validation of the final candidate.

- [x] Final release commit passes macos-15 arm64 and macos-15-intel CI
- [ ] Final signed release bundle: light/dark appearance, rounded corners, no
  clipping, stable numeric/dot badge, READY, hover and Return/Open App
- [ ] Real v0.3 → v0.4 upgrade: brew upgrade → agentbell install → App running;
  existing config/hooks preserved and SQLite healthy
- [ ] Real stale session: close a working agent terminal/tab or kill that test
  agent without Stop; its WORKING row must eventually disappear
- [ ] Four real agents, Badge 1 → 0, actual menu Return and deliberate-Quit suppression / crash fallback
  notification click loop (detailed below)

## Historical verified release checks

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
- [ ] NEEDS YOU / WORKING / READY visual appearance and ordering
- [ ] Return visible and clickable
- [ ] Pause / Resume / Clear Ready
- [ ] No Dock icon; basic keyboard operation
- [ ] Tabby Claude A WORKING / Claude B NEEDS YOU / Claude C DONE; GoLand Codex D WORKING
- [ ] Badge 1, B needs attention, A/D working, C READY
- [ ] B answers/resumes: NEEDS YOU → WORKING, Badge 1 → 0
- [ ] Actual Menu Bar Return click to correct Tabby session
- [ ] Actual Menu Bar Return click to correct GoLand session
- [ ] Select Quit AgentBell, trigger an actual Claude Stop/NeedsInput; no new
  notification appears and the App does not relaunch
- [ ] Open App again; real subsequent events are received (existing Pause preserved)
- [ ] Involuntary App exit, trigger actual Claude Stop/NeedsInput; fallback
  system notification appears; click returns according to existing capability

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

### Natural-height small lists and smaller numeric badge

The user clarified that the numeric badge also needed scaling. The count capsule
now measures 10 points high rather than 12, with an 8-point single-digit glyph;
the completion dot decreases from 5 to 3.5 points. The previous geometry preference
approach still allowed half-clipped summary text in the user's screenshot.
Small lists (up to two visible sessions) now render at their natural full height
without a ScrollView or asynchronous measurement loop. Longer lists retain a
bounded scroll viewport. Panel width remains 460 points. Notification defaults
are unchanged. Both architectures build and bundle smoke passes; real visual
acceptance remains for the user after the requested direct replacement.

### Quiet Done, per-row removal, integration readiness and packaged Return fix

Codex restart was manually confirmed by the user: a genuine Codex Done appeared
in RECENT. System notification history is separate from current session state.
The resident App now defaults to quiet completions (RECENT + unseen dot), with
attention/error banners retained. `attention_center.done_notifications=true`
opts back into resident completion banners. Quit / CLI-only notification fallback
retains `notifications.done`; past notifications are not deleted automatically.

RECENT has a per-row × using validated `remove_recent SESSION_ID` IPC. Only
completed, resolved sessions can be deleted. Installation explicitly requests
agent/plugin reload where appropriate and distinguishes written hook config from
confirmed event delivery. Agent Connections reports events during this App run,
never inferring delivery from restored database state. Hook paths now point to
the fixed installed bundle rather than each development extraction directory.

A real Return click logged `surface=agentbell`: GenericProvider inspected the
packaged hook executable's own .app before reaching its parent Codex bundle.
It now skips AgentBell transport bundles and continues process ancestry. Exact
Tabby/GoLand/tmux detection is unchanged. App/project-only return controls are
labeled Open App/Open Project. Return failures keep the panel and error visible.
Desktop Codex return remains application-level; exact chat navigation is not
implemented or claimed. Tests cover parent-bundle selection, selective deletion
and persistence, real-event readiness, and quiet resident / audible fallback
policy. All visual/notification/Return actions still need genuine manual review.

### Correct idle semantics, compact menu spacing and removal feedback

Real user feedback identified normal Claude turn completion as false NEEDS YOU.
The official Notification reference defines `idle_prompt` as a completed response
with no new user input for about 60 seconds:
https://code.claude.com/docs/en/hooks#notification
The adapter now represents it as an idle event, and the engine leaves state
unchanged: Stop retains its real completion time and summary, idle adds neither
attention nor new unread completion and cannot recreate cleared RECENT history.
Real `agent_needs_input`, elicitation forms/URLs and approval prompts retain their
input/approval classification. Regression tests cover Stop → idle, real waits,
and idle after Clear Recent.

The status image no longer reserves badge canvas width when no badge is visible
(22 points normally, 26 with count/dot). The per-row × has animated hover fill /
contrast and pressed fill / scale feedback. The confirmed local legacy false
idle row is repaired only against its matching persisted Stop transition after
the App/core shuts down, with a local database backup; no other wait is reclassified.


### Current visual follow-up candidate

- The status icon uses the same 22-point canvas with and without badges; numeric
  badges and the unseen-completion dot overlay the glyph without moving it.
- Explicit session titles are preferred for rows and notifications. Codex desktop
  uses its local thread catalog; Claude uses its optional session index. Unknown
  schemas or missing titles fall back to the project.
- The visible RECENT label is now READY / Ready to continue, representing a
  completed turn rather than an archived conversation. IPC/storage retain recent.
- The visual-effect background uses an explicit rounded alpha mask, updated with
  panel sizing; the hosting layer is also clipped and the shadow invalidated.
  Actual corner appearance still requires manual visual acceptance.


### Deliberate Quit behavior (user decision supersedes original gate)

Quit now disables hooks' event delivery and new notifications until the next App
launch, using a persisted user-disabled marker. Crashes, upgrade shutdown and
CLI-only first use keep basic fallback. Footer bell is the single Pause/Resume
entry; the duplicate settings item has been removed. Previously delivered system
notifications remain in macOS history. The original Quit→notify gate is replaced
by deliberate-Quit silence plus involuntary-exit fallback acceptance.


### Live elapsed time

Each session's elapsed-time label now uses a one-second TimelineView and its
scheduled date. Only the status text refreshes; timestamps remain event-based.
The original s → m → h → d unit thresholds are preserved; only the refresh
interval changes from fifteen seconds to one second. Manual acceptance should
confirm successive seconds while the panel is open and correct elapsed time
after reopening or waking the machine.


Actual Codex hooks/list diagnostics found all six installed AgentBell hooks
enabled but trustStatus=modified; no current WORKING events reached AgentBell.
User review/trust in Codex is required, followed by real event verification.
Installation now prints that prerequisite. No trust state was modified by AgentBell.

After the user reviewed/trusted the modified hooks, actual Codex Working and
ToolActivity events arrived for the current conversation and it appeared under
WORKING. No synthetic event or Codex restart was used to verify recovery.


### Publication authorization — 2026-10-06

The user explicitly requested publication of the current version. Proceed with
final-commit dual-architecture CI, tag/Release and Homebrew upgrade checks. This
authorization does not mark the outstanding manual checklist as passed; release
notes retain the incomplete manual validation scope.


## Published result — 2026-10-06

- Release source: be3ff09dddcc410397a3b51a6b338a8254dc608d (current-turn Working age fix).
- [Final source CI](https://github.com/Han1enG/agent-bell/actions/runs/37413189256): arm64/Intel passed.
- [Tag and publishing CI](https://github.com/Han1enG/agent-bell/actions/runs/37413531326): both test jobs and release job passed.
- [GitHub Release](https://github.com/Han1enG/agent-bell/releases/tag/v0.4.0): published, not draft/prerelease.
- Exact downloaded published archives passed checksum, signature and release smoke checks.
- Tap main commit 428b288 uses those exact published hashes.
- Real Homebrew 0.3.0 → 0.4.0 upgrade and agentbell install completed; version and brew test passed.
- config.toml, Codex hooks.json and Claude settings.json hashes preserved.
- Installed App matches the Homebrew App executable; App and owned core running,
  IPC protocol 1, real Codex Working events received, SQLite integrity ok/schema 1.
- Doctor is NOT fully green: notifications report not requested yet, and the
  GoLand bridge is currently unreachable. Tabby bridge is reachable.
- Remaining manual checks are not retroactively marked passed.

Published arm64 SHA256: a18845d1e6f43c05d32806e22fe6e2edfe7202b84f0302dc04caca46c7029d06

Published amd64 SHA256: 5ffa177fc716c852fd3a6992d62df2a87f519797380601156d9343517fff2df7
