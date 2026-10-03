# AgentBell Context Protocol v1

Status: **Frozen v1** for v0.3. Breaking identity, probe, focus, payload or fallback changes require a new protocol version. Optional discovery maturity fields do not change the return contract. Legacy v0.2 targets without Layers retain the single-provider path.

## Overview

AgentBell integrations identify the context in which an agent runs, expose a
read-only liveness probe, and focus that same context when a notification is
clicked. All communication is local. v1 defines a contract, not a dynamic plugin
loader or an arbitrary command execution mechanism.

## Environment Variables

| Variable | Meaning |
|---|---|
| `AGENTBELL_SURFACE` | Registered provider name, e.g. `tabby` or `jetbrains` |
| `AGENTBELL_CONTEXT_ID` | Opaque provider-owned context identity |

An integration supplies both variables to **new local terminals**, independently
for each terminal. Do not mutate global shell state or shared profiles, inject
commands into running terminals, or export local bridge IDs into SSH sessions.
Core passes ContextID unchanged. It does not interpret delimiters, UUIDs, JSON or
provider-specific metadata. Native providers may use their own environment, e.g.
`TMUX_PANE`, `ITERM_SESSION_ID`, `WEZTERM_PANE`, `WEZTERM_UNIX_SOCKET`.
Unknown provider names retain basic app/project fallback; an ID alone never
creates a provider or grants exact return.

## Context Lifecycle

IDs must remain stable while the context lives, uniquely identify that context,
expire on closure, and not identify a new context after app/server restart.
Use random instance + context generations or native stable IDs plus independently
verified process/socket generation. Array positions, pane indexes, cwd, titles
and profile names are not identities. Never replace an expired target with a
similar one. Restored PTYs must acquire fresh provider identity when needed.

## Probe Contract

`Probe(ReturnTarget)` is read-only. It may enumerate context identities and check
local IPC/process/socket identity. It must not activate an app, switch a tab,
change selection, create sessions, read terminal contents or capture screens.
The legacy Go interface returns a typed failure; the core `surface.Probe` adapter
provides the standard result:

- `valid`: the exact context and relevant instance are live and focusable.
- `expired`: context/pane is gone or instance/server generation changed.
- `unreachable`: bridge/provider/app unavailable, permission unavailable.
- `unsupported`: no safe focus capability for this surface/context.

`ProbeResult` includes `status`, `valid`, `capability` and optional `reason`.
Capabilities are conditional: `valid` is not inferred from installation or an ID.
CLI discovery/doctor do not request Automation consent. With no native helper or
permission, they report unavailable; notification clicks may request normal
macOS consent.

## Focus Contract

`Return(ReturnTarget)` may activate the originating app and select an existing
window/tab/pane/session. Revalidate immediately before mutation; resolve the
exact native ID at focus time. Never execute shell commands in a terminal, send
input, read terminal content, kill/create sessions or change layout. Failure must
remain observable. `surface.Focus` adapts legacy providers to `ReturnResult`
(`Success`, achieved `Capability`, typed `Reason`). Legacy error-based interfaces
are retained for source compatibility with the shipped integrations.

## Capabilities

Providers declare `Capabilities()` from `app`, `window`, `exact_context` and
`project`. The generic fallback performs app/project return on behalf of enhanced
providers. Declare exact only with unique identity, read-only probe, safe focus,
and generation protection. Do not declare a separate window capability just
because exact focus happens to bring a window forward.

## Composite Contexts

ReturnTarget preserves v0.2 fields and adds optional `Metadata` and ordered
`Layers`. Each layer has `Provider`, `ContextID`, `WindowID`, `Metadata` and its
own capability. v0.3 executes one outer surface plus tmux. The notification stores
the layers; core rebuilds a fixed app → outer context → inner context plan.
It never executes serialized commands. All critical IDs are reprobed on click.
Provider-specific metadata is opaque; attachment proof comes from provider-owned
`ClientTTY` / `VerifyClient` methods, not an untrusted metadata claim.

For tmux, exact composite return requires one unchanged attached client and a
verified match between its TTY and the outer context. Multiple clients, detached
clients, mismatching inherited environment and unsupported binding fall back to
app/project. Providers that cannot prove this relationship must not promise
exact tmux return merely because they can select a pane in a server.

## Security Requirements

Register trusted, installed providers in `internal/surface/builtin`. No binary,
command, script, shell expression, plugin URL or arbitrary action in a notification
payload is executable authority. Commands use fixed executables and argument
arrays; IDs are data, never interpolated into script source. Bridges are local
Unix sockets in user-owned private directories. Validate socket ownership and
generation; limit messages and timeouts. Provider panics/failures are isolated
from app/project fallback. No telemetry, uploads, analytics or terminal content
collection. No runtime download/loading system is part of v1.

## Fallback Semantics

Exact → supported window → originating app → validated project directory.
Normal context closure is an expected stale-context outcome, not a program bug.
Reasons include `context_not_found`, `pane_not_found`, `server_identity_mismatch`,
`instance_mismatch`, `multiplexer_unreachable`, `bridge_unreachable`,
`provider_unavailable`, `permission_denied`, `app_not_running`, `invalid_target`,
`invalid_cwd`, `unsupported_surface`, `unknown`. Never guess by cwd/title/name.

## Example Integration

See [the minimal provider sketch](provider-example/README.md). Tabby's bundled
bridge is a working local implementation; its IDs expire with the bridge window.
Third-party integrations implement the public contracts without special-case
logic in main.go. v0.3 still requires trusted compile/install-time registration.
