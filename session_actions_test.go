package main

import (
	"errors"
	"github.com/han1eng/agent-bell/internal/surface"
	"testing"
)

type strictProvider struct {
	name                string
	probeErr, returnErr error
	returned            int
}

func (p *strictProvider) Name() string { return p.name }
func (p *strictProvider) Detect(surface.DetectContext) (*surface.ReturnTarget, error) {
	return nil, nil
}
func (p *strictProvider) CanHandle(surface.ReturnTarget) bool { return true }
func (p *strictProvider) Probe(surface.ReturnTarget) error    { return p.probeErr }
func (p *strictProvider) Return(surface.ReturnTarget) error   { p.returned++; return p.returnErr }
func TestMenuReturnDoesNotSilentlyFallback(t *testing.T) {
	p := &strictProvider{name: "test", probeErr: errors.New("expired")}
	r := surface.Registry{}
	r.Register(p, 0, false)
	target := surface.ReturnTarget{Surface: "test", Capability: surface.ReturnExactContext, ContextID: "old", AppBundleID: "com.test.app"}
	if err := returnSessionContext(r, target); err == nil || p.returned != 0 {
		t.Fatal("expired target accepted or focused")
	}
	p.probeErr = nil
	p.returnErr = errors.New("closed between probe and focus")
	if err := returnSessionContext(r, target); err == nil || p.returned != 1 {
		t.Fatal("focus race silently succeeded")
	}
	p.returnErr = nil
	if err := returnSessionContext(r, target); err != nil {
		t.Fatal(err)
	}
}
