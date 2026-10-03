package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

type discoveryProvider struct{ valid bool }

func (discoveryProvider) Name() string { return "fixture" }
func (discoveryProvider) Detect(surface.DetectContext) (*surface.ReturnTarget, error) {
	return nil, nil
}
func (discoveryProvider) CanHandle(t surface.ReturnTarget) bool { return t.Surface == "fixture" }
func (discoveryProvider) Return(surface.ReturnTarget) error     { panic("discovery must not focus") }
func (p discoveryProvider) Probe(surface.ReturnTarget) error {
	if p.valid {
		return nil
	}
	return surface.Fail(surface.ContextNotFound, errors.New("expired"))
}
func (discoveryProvider) Capabilities() []surface.ReturnCapability {
	return []surface.ReturnCapability{surface.ReturnApp, surface.ReturnExactContext}
}
func TestDiscoverySeparatesInstalledCurrentAndLive(t *testing.T) {
	for _, valid := range []bool{false, true} {
		var r surface.Registry
		r.Register(discoveryProvider{valid}, 1, false)
		d := discover(r, surface.ReturnTarget{Surface: "fixture", ContextID: "opaque", AppBundleID: "app", Capability: surface.ReturnExactContext}, func(string) bool { return false })
		if !d.Providers[0].Detected || d.Providers[0].Installed || d.Current.Layers[0].Probe.Valid != valid {
			t.Fatal(d)
		}
		if (d.Current.Capability == surface.ReturnExactContext) != valid {
			t.Fatal(d)
		}
	}
}
func TestSurfacesJSONIsPureAndStable(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("AGENTBELL_SURFACE", "")
	t.Setenv("TMUX", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("ITERM_SESSION_ID", "")
	var b bytes.Buffer
	if err := run([]string{"surfaces", "--json"}, strings.NewReader(""), &b, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var d discovery
	if err := json.Unmarshal(b.Bytes(), &d); err != nil || d.SchemaVersion != 1 || d.Current.Inner == nil || len(d.Providers) < 7 {
		t.Fatal(err, b.String())
	}
	if surfacesCommand([]string{"--bad"}, &b) == nil {
		t.Fatal("invalid option")
	}
}
