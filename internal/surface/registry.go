package surface

import (
	"errors"
	"os"
	"sort"
	"time"
)

// Layer order is outer → inner. The outer target retains its own capability.
type SurfaceLayer struct {
	Provider   string
	ContextID  string
	WindowID   string            `json:",omitempty"`
	Metadata   map[string]string `json:",omitempty"`
	Capability ReturnCapability
}

func (l SurfaceLayer) Target(parent ReturnTarget) ReturnTarget {
	parent.Layers = nil
	parent.Surface, parent.ContextID, parent.WindowID = l.Provider, l.ContextID, l.WindowID
	parent.Metadata, parent.Capability = l.Metadata, l.Capability
	return parent
}
func Layer(t ReturnTarget) SurfaceLayer {
	return SurfaceLayer{t.Surface, t.ContextID, t.WindowID, t.Metadata, t.Capability}
}

type ProbeResult struct {
	Status     string           `json:"status"`
	Valid      bool             `json:"valid"`
	Capability ReturnCapability `json:"capability"`
	Reason     FailureReason    `json:"reason,omitempty"`
}
type ReturnResult struct {
	Success    bool
	Capability ReturnCapability
	Reason     FailureReason
}

func Probe(p SurfaceProvider, t ReturnTarget) ProbeResult {
	result := ProbeResult{Status: "unsupported", Capability: t.Capability, Reason: UnsupportedSurface}
	handles := false
	handleErr := Isolate(func() error { handles = p.CanHandle(t); return nil })
	if handleErr != nil {
		result.Status = "unreachable"
		result.Reason = ProviderUnavailable
		return result
	}
	if !handles {
		return result
	}
	probe, ok := p.(ProbeableProvider)
	if !ok {
		return result
	}
	err := Isolate(func() error { return probe.Probe(t) })
	result.Valid, result.Reason = err == nil, Reason(err)
	if err == nil {
		result.Status = "valid"
	} else if result.Reason == ContextNotFound || result.Reason == PaneNotFound || result.Reason == ServerIdentityMismatch || result.Reason == InstanceMismatch {
		result.Status = "expired"
	} else if result.Reason != UnsupportedSurface {
		result.Status = "unreachable"
	}
	return result
}
func Focus(p SurfaceProvider, t ReturnTarget) ReturnResult {
	r := Probe(p, t)
	if !r.Valid {
		return ReturnResult{Reason: r.Reason}
	}
	err := Isolate(func() error { return p.Return(t) })
	return ReturnResult{Success: err == nil, Capability: t.Capability, Reason: Reason(err)}
}
func Isolate(fn func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = Fail(ProviderUnavailable, errors.New("provider panic"))
		}
	}()
	return fn()
}

type CapabilityProvider interface{ Capabilities() []ReturnCapability }

func Capabilities(p SurfaceProvider) []ReturnCapability {
	if c, ok := p.(CapabilityProvider); ok {
		return c.Capabilities()
	}
	return nil
}
func (GenericProvider) Capabilities() []ReturnCapability {
	return []ReturnCapability{ReturnApp, ReturnProject}
}

type registration struct {
	provider SurfaceProvider
	priority int
	inner    bool
}
type Registry struct{ entries []registration }

func (r *Registry) Register(p SurfaceProvider, priority int, inner bool) {
	for _, e := range r.entries {
		if e.provider.Name() == p.Name() {
			panic("duplicate provider: " + p.Name())
		}
	}
	r.entries = append(r.entries, registration{p, priority, inner})
	sort.SliceStable(r.entries, func(i, j int) bool { return r.entries[i].priority > r.entries[j].priority })
}
func (r Registry) Providers() []SurfaceProvider {
	var ps []SurfaceProvider
	for _, e := range r.entries {
		ps = append(ps, e.provider)
	}
	return ps
}
func (r Registry) Provider(name string) SurfaceProvider {
	for _, e := range r.entries {
		if e.provider.Name() == name {
			return e.provider
		}
	}
	return nil
}
func (r Registry) Detect(c DetectContext) *ReturnTarget {
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.Deadline.IsZero() {
		c.Deadline = time.Now().Add(90 * time.Millisecond)
	}
	innerStarted := time.Now()
	var inner *ReturnTarget
	for _, e := range r.entries {
		if e.inner {
			_ = Isolate(func() error { var err error; inner, err = e.provider.Detect(c); return err })
			if inner != nil {
				break
			}
		}
	}
	if c.OnTiming != nil {
		c.OnTiming("tmux_detect", time.Since(innerStarted))
	}
	outerContext := c
	// The attached client's ancestry identifies the GUI, not the detached server.
	if inner != nil {
		if origin, ok := r.Provider(inner.Surface).(AttachmentOriginProvider); ok {
			pid, err := origin.ClientPID(*inner)
			if err == nil && pid > 1 {
				outerContext.PID = pid
			}
		}
	}
	var outer *ReturnTarget
	for _, e := range r.entries {
		if e.inner {
			continue
		}
		err := Isolate(func() error { var err error; outer, err = e.provider.Detect(outerContext); return err })
		if err == nil && outer != nil {
			break
		}
		outer = nil
	}
	if outer == nil {
		outer = &ReturnTarget{Surface: "generic", CWD: c.CWD, Capability: ReturnProject}
	}
	if inner == nil && c.Env("TMUX") != "" {
		outer.ContextID = ""
		outer.WindowID = ""
		outer.Capability = ReturnApp
		if outer.AppBundleID == "" {
			outer.Capability = ReturnProject
		}
	}
	if inner != nil {
		outer.Layers = []SurfaceLayer{Layer(*outer), Layer(*inner)}
		// Exact inner focus also needs evidence that the selected GUI context owns
		// the recorded attached client TTY. Providers can supply this read-only proof.
		if bound, ok := r.Provider(outer.Surface).(ClientBindingProvider); ok && outer.Capability == ReturnExactContext && inner.Capability == ReturnExactContext && Isolate(func() error { return detectionBinding(bound, c, *outer, clientTTY(r.Provider(inner.Surface), *inner)) }) == nil {
			outer.Capability = ReturnExactContext
		} else {
			outer.Capability = ReturnApp
			if outer.AppBundleID == "" {
				outer.Capability = ReturnProject
			}
		}
	}
	return outer
}

type DetectionBindingProvider interface {
	VerifyClientDuringDetection(DetectContext, ReturnTarget, string) error
}

func detectionBinding(p ClientBindingProvider, c DetectContext, t ReturnTarget, tty string) error {
	if p, ok := p.(DetectionBindingProvider); ok {
		return p.VerifyClientDuringDetection(c, t, tty)
	}
	return p.VerifyClient(t, tty)
}

type AttachmentOriginProvider interface {
	ClientPID(ReturnTarget) (int, error)
}
type AttachedClientProvider interface {
	ClientTTY(ReturnTarget) (string, error)
}

func clientTTY(p SurfaceProvider, t ReturnTarget) string {
	if p, ok := p.(AttachedClientProvider); ok {
		tty, err := p.ClientTTY(t)
		if err == nil {
			return tty
		}
	}
	return ""
}

type ClientBindingProvider interface {
	VerifyClient(ReturnTarget, string) error
}

type ReturnStep struct {
	Provider string
	Target   ReturnTarget
}
type ReturnPlan struct{ Steps []ReturnStep }

func BuildPlan(t ReturnTarget) ReturnPlan {
	plan := ReturnPlan{}
	app := t
	app.Layers = nil
	app.Capability = ReturnApp
	app.CWD = ""
	if app.AppBundleID != "" {
		plan.Steps = append(plan.Steps, ReturnStep{"generic", app})
	}
	layers := t.Layers
	if len(layers) == 0 && t.Capability != ReturnApp {
		layers = []SurfaceLayer{Layer(t)}
	}
	for _, l := range layers {
		plan.Steps = append(plan.Steps, ReturnStep{l.Provider, l.Target(t)})
	}
	return plan
}
func (m Manager) provider(name string) SurfaceProvider {
	for _, p := range m.Providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
func (m Manager) returnComposite(t ReturnTarget) error {
	if len(t.Layers) != 2 || t.Layers[1].Provider != "tmux" {
		return Fail(InvalidTarget, errors.New("unsupported return stack"))
	}
	plan := BuildPlan(t)
	outer := plan.Steps[len(plan.Steps)-2].Target
	// Activate outer app first, then independently attempt precise outer return.
	app := outer
	app.ContextID = ""
	app.WindowID = ""
	app.Capability = ReturnApp
	app.CWD = ""
	appOK := false
	if app.AppBundleID != "" {
		appOK = m.ReturnToContext(app) == nil
	}
	inner := plan.Steps[len(plan.Steps)-1].Target
	innerProvider := m.provider(inner.Surface)
	outerProvider := m.provider(outer.Surface)
	binding, bindingOK := outerProvider.(ClientBindingProvider)
	outerOK := false
	innerProbe := ProbeResult{Status: "unsupported", Reason: ProviderUnavailable}
	if innerProvider != nil {
		innerProbe = Probe(innerProvider, inner)
	}
	innerLive := innerProbe.Valid
	if !innerLive && m.OnAttempt != nil {
		m.OnAttempt(inner.Surface, inner.Capability, Fail(innerProbe.Reason, errors.New("inner probe failed")))
	}
	// Prove the attachment before focusing the outer context. Inherited protocol
	// variables from a detached/re-attached server must never select an old tab.
	bound := false
	if innerLive && bindingOK {
		bindingErr := Isolate(func() error { return binding.VerifyClient(outer, clientTTY(innerProvider, inner)) })
		bound = bindingErr == nil
		if bindingErr != nil && m.OnAttempt != nil {
			m.OnAttempt(outer.Surface, outer.Capability, bindingErr)
		}
	}
	if bound && outerProvider != nil && outer.Capability == ReturnExactContext {
		result := Focus(outerProvider, outer)
		outerOK = result.Success
		if m.OnAttempt != nil {
			var err error
			if !outerOK {
				err = Fail(result.Reason, errors.New("outer focus failed"))
			}
			m.OnAttempt(outerProvider.Name(), outer.Capability, err)
		}
	}
	if outerOK && bound {
		result := Focus(innerProvider, inner)
		if m.OnAttempt != nil {
			var err error
			if !result.Success {
				err = Fail(result.Reason, errors.New("inner focus failed"))
			}
			m.OnAttempt(innerProvider.Name(), inner.Capability, err)
		}
		if result.Success {
			return nil
		}
	}

	if appOK || outerOK {
		return nil
	}
	outer.ContextID = ""
	outer.WindowID = ""
	return m.ReturnToContext(outer)
}
