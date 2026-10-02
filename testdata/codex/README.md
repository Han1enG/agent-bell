# Codex fixture provenance

`permission-request.json` and `stop.json` are inherited, minimized documentation
examples. They are **not captures from a live Codex hook**. Real user-required,
auto-accepted, Stop and failure captures remain outstanding. Do not relabel
examples or create synthetic payloads to fill that gap.

`approval-auto-review.json` is a real desktop Codex capture from 2026-10-02
(Asia/Shanghai), runtime version 0.159.2, with Auto-review enabled. It was
triggered by an escalated local `go test -count=1 ./internal/adapter` invocation.
Auto-review allowed the command without a human approval prompt; the existing
AgentBell notification handler received `PermissionRequest` beforehand.

The existing AgentBell executable launcher was temporarily wrapped to capture
stdin and forward the original bytes to the original executable. Approval
policy and hook decisions were unchanged. The launcher was restored to its
original symlink immediately after capture. The tap may affect process ancestry
during that short interval, so this fixture is approval evidence only.

Session/turn IDs, transcript path, model, command and description strings were
redacted; cwd was replaced with an example path. Fields and structure were kept.
The actual input says `permission_mode: default` and has no final reviewer,
decision or human-wait field. Observed Auto-review outcome is provenance, not
an invented payload field.

The current public `PermissionRequest` input runs before approval routing. It
has no documented final reviewer/decision/human-wait field. `permission_mode`
is not a substitute for that missing signal. AB-001 remains open.

For an opt-in capture in a fresh Codex session, add a separate command handler
pointing to `scripts/capture-codex-hook.py`, passing an absolute private output
directory. Keep all existing hooks and approval/security policy unchanged. The
tap writes no stdout, makes no approval decision, redacts free-text strings and
preserves object/array structure, field names, booleans and numeric values.
Capture failures do not block the operation. Remove the tap after collection.

Before committing a capture, review redaction and add provenance with:

- Codex version, capture date, desktop/CLI surface and effective reviewer mode.
- How the event was triggered, whether an actual human prompt appeared, and the
  observed outcome. Do not insert these observations into the captured payload.
- Exactly which values were redacted; preserve event/state fields.

Only then name the file `approval-user-required.json`,
`approval-auto-review.json`, or `approval-auto-accepted.json` according to the
observed run. Identical pre-decision payloads from different outcomes must stay
identical; downstream observation must never be invented as an input field.
