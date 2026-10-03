# Minimal integration sketch

MyTerminal generates a fresh application-instance UUID on startup and a fresh
context UUID for every new local terminal. It injects these two variables when
creating that terminal (without sending input):

```text
AGENTBELL_SURFACE=myterminal
AGENTBELL_CONTEXT_ID=<opaque instance + context ID>
```

Implement a private local bridge in the application:

```text
list / probe(id):
  check ID belongs to this live app instance and a live focusable terminal
  return valid, expired, unreachable or unsupported
  do not change selection or read terminal text

focus(id):
  resolve the same live ID again
  if absent: return context_not_found
  activate app, select existing window/tab, focus existing terminal
  return achieved capability
```

Compile-time provider sketch (provider interprets the ID, core does not):

```go
func (Provider) Name() string { return "myterminal" }
func (Provider) Capabilities() []surface.ReturnCapability {
    return []surface.ReturnCapability{surface.ReturnApp, surface.ReturnExactContext}
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
    if c.Env("AGENTBELL_SURFACE") != p.Name() { return nil, nil }
    // Start with verified originating app identity. Copy ContextID unchanged.
    // Upgrade only with reliable instance/context evidence; fall back otherwise.
    return p.detectOriginAndContext(c)
}
func (Provider) CanHandle(t surface.ReturnTarget) bool {
    return t.Surface == "myterminal" && t.Capability == surface.ReturnExactContext
}
func (p Provider) Probe(t surface.ReturnTarget) error { return p.bridgeProbe(t.ContextID) }
func (p Provider) Return(t surface.ReturnTarget) error { return p.bridgeFocus(t.ContextID) }
// Trusted registration in builtin.Registry, ahead of generic:
// r.Register(myterminal.Provider{}, 70, false)
```

For tmux composition, additionally implement `VerifyClient(target, tty)` using
a read-only proof that the target terminal owns that TTY. Do not assume that
inherited protocol variables still refer to the current tmux attachment.
The sketch's helper methods are intentionally pseudocode; no external command
from a payload is executed and no dynamic plugin installation is implied.
