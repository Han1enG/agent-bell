# AgentBell v0.4 — implementation and release report

Date: 2026-10-05 (Asia/Shanghai)

Status: **v0.4.0 published on 2026-10-06** from be3ff09. Final source and tag CI passed on arm64 and Intel; published archives and real Homebrew upgrade verified. See RELEASE_ACCEPTANCE_V04.md for the final result and outstanding manual checks.

The implementation detail and RC results below describe earlier candidates, not the final publication record.

## Delivered

- Native SwiftUI/AppKit AgentBell.app, menu bar only, custom template bell, attention count, NEEDS YOU / WORKING / RECENT, bounded summaries, explicit Return buttons, capability tooltips and accessibility labels.
- App-owned Go session engine, source-namespaced official IDs and process/context fallback IDs, independent attention state/timestamp, deterministic ordering, resumed-work clearing, observed permission safety and stale archival.
- Private versioned Unix IPC, bounded readers/concurrency, 40ms total hook deadline and existing notification fallback. State updates precede notification configuration, debounce and pause.
- SQLite v1 migration, system sqlite3 via CGO, snapshots, bounded minimal transition journal, seven-day retention, pause persistence, restore and read-only CLI fallback. No raw hook payloads/prompts/tool arguments stored. Storage failure keeps memory tracking operational.
- Pause/Resume, Clear Recent, Show More, Open Config, status/text + pure JSON, version components + --short, bounded Go logs/follow, app/store/version/login doctor diagnostics, doctor repair, zero-write dry-run and CLI-only mode.
- Existing Universal Return registry reused through the bundled CLI. No second focus implementation. Existing user summary-filter changes preserved.
- Staged app installation/upgrade, app-only graceful shutdown, default login launch, owned hook updates and preserved config. Homebrew formula staged against actual local artifact checksums. CI adds v0.4 builds, packages and synthetic native lifecycle acceptance on both runner architectures.

## Verified locally

| Check | Result |
| --- | --- |
| Existing v0.3 Go baseline | Passed outside sandbox; Unix sockets/process probes require it |
| Full Go regression + race detector | Passed, including final source changes |
| go vet and diff whitespace checks | Passed |
| State creation/update/merge and transition tests | Passed |
| Same session repeated NeedsInput | Badge remains 1; original attention age retained |
| Two waiting Claude + working Codex | Badge 2 |
| PermissionRequest safety | Codex request never changes attention; Claude pre-routing request never creates attention |
| Pause and notification config independence | Tracking continues |
| Recent sorting/retention/clear/stale handling | Passed |
| SQLite v1 migration, restore, privacy, permissions, future-version rejection | Passed |
| Missing/wedged IPC, duplicate owner, major compatibility, unknown fields | Passed |
| Hook→IPC and missing-app native helper fallback | Passed with captured helper invocation, without posting a real alert |
| Storage failure | Current memory state remains readable |
| Install upgrade, login preference and config preservation | Passed in isolated fixtures |
| Tabby Node bridge regression | 6/6 passed, including real local Unix bridge sockets |
| JetBrains Java↔Go regression | Passed using installed GoLand JBR; isolated bridge instances |
| Existing Python regression | 3/3 passed |
| arm64 + amd64 packages and codesign deep/strict verification | Passed; ad-hoc signing |
| Exact final package release smoke | Passed, including correct GUI executable, pure JSON and read-only empty status |
| Actual signed arm64 native app with synthetic hooks | Passed: A/D working, B input attention, C recent; state count 1 |
| Actual app pause→B resume | Count clears to 0 while pause remains enabled |
| Actual app/core termination, offline DB restore and App restart | Passed; no surviving app-owned core/socket |
| Actual App pause persistence, Clear Recent, Resume and CLI-only | Passed |
| Actual app after periodic reconciliation timer | Isolated 65-second run passed |
| IPC event+ack, 100 requests to actual App-owned core | median 0.038ms; p95 0.102ms; max 0.451ms (local sample, not a GUI latency measurement) |
| Homebrew formula | Ruby syntax passes; hashes match the exact final local packages |

Local native builds used the installed macOS 15.4 SDK explicitly. The machine's default macOS 27 SDK produced unsupported architecture errors in its linker. This is a local toolchain compatibility issue, not a package dependency. Build scripts accept SDKROOT.

## Outstanding acceptance / release gates

- UI automation could not read the native menu because the computer-use server timed out. Visual layout, actual displayed badge, keyboard behavior, and clicking a menu Return button are **not** recorded as verified. The interrupted UI-discovery run also had a failed resume assertion; isolated runs without UI discovery passed, including a 65-second timer run and final signed package lifecycle run. Its exact cause remains unproven, so visual/live acceptance is still a release gate.
- Real simultaneous Claude/Codex agents in three Tabby tabs plus GoLand, actual human input and exact live Return still need the prescribed manual acceptance. Synthetic events prove transport/state, not upstream human-wait detection.
- Codex's current official hooks lack an input-wait notification. A reliable explicitly supplied needs_input event is supported, but PermissionRequest cannot substitute. Do not claim automatic detection of every actual Codex wait.
- Real system notification delivery/clicks after App quit and cold launch, including notifications posted by v0.3, require macOS acceptance. Tests capture the existing helper path; no real notification permission prompt was accepted during this work.
- tmux E2E could not run because tmux is not installed. Existing Go tmux regressions passed; CI installs tmux and retains its E2E.
- Login after a real login/physical machine restart remains untested.
- amd64 is built and signature-verified locally; execution and native lifecycle validation on Intel are delegated to the updated CI job, which has not run remotely yet.
- Signing is ad-hoc. No Developer ID signing/notarization is claimed.
- Release assets, Git tag, remote CI, and Homebrew tap changes have not been pushed/published. Formula URLs will become usable only after publishing the exact matching packages. The formula and updater are staged locally; do not publish a changed archive under these hashes.

## Exact local artifacts

Under `dist/release/` (ignored build outputs):

- `agentbell_0.4.0_darwin_arm64.tar.gz`: `e8361a0812ae705b2a04b8751d9acda21e5fa8c11d01a3d3857696085b134259`
- `agentbell_0.4.0_darwin_amd64.tar.gz`: `cc022f2544ced115ac417fbd10c4e8ee4f872601ada2ccf97e01afdc13f391f5`

The staged tap is `../homebrew-agentbell/Formula/agentbell.rb`. If rebuilding, rerun `scripts/update-homebrew.py` and replace the hashes in this report before publication.

## Repeatable checks

```sh
go test -race -count=1 ./...
go vet ./...
node --test integrations/agentbell-tabby/bridge.test.js integrations/agentbell-tabby/context.test.js
python3 -m unittest discover -s scripts -p '*_test.py'
python3 integrations/agentbell-jetbrains/test_bridge.py
./scripts/build-macos.sh
./scripts/release-smoke.sh dist/release
./scripts/release-attention-e2e.sh dist/release
python3 scripts/update-homebrew.py ../homebrew-agentbell/Formula/agentbell.rb
```

Sandboxed execution needs permission for Unix socket/process tests and a writable GOCACHE. An isolated visual-QA harness is available as `scripts/attention-ui-e2e.py /absolute/path/AgentBell.app --hold 180`; it changes only temporary synthetic sessions and terminates its own App afterward.

No v0.5 work was started.
