package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/surface"
)

func resumeProviderFixture(t *testing.T) (TabbyResumeProvider, attention.ResumeTarget) {
	t.Helper()
	home := t.TempDir()
	for _, relative := range []string{"Applications/Tabby.app/Contents/MacOS/Tabby", "Applications/AgentBell.app/Contents/MacOS/agentbell", ".local/bin/claude"} {
		path := filepath.Join(home, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	s := attention.Session{ID: "claude:12345678-1234-1234-1234-123456789abc", Agent: "claude", NativeSessionID: "12345678-1234-1234-1234-123456789abc", AgentFlavor: "claude_cli", RuntimeState: attention.RuntimeExited, RuntimeInstanceID: "old", CWD: home}
	target, err := attention.ResumeArguments(s, filepath.Join(home, ".local/bin/claude"))
	if err != nil {
		t.Fatal(err)
	}
	return TabbyResumeProvider{Home: home, Session: s, Timeout: time.Millisecond}, target
}

func TestTabbyLaunchAloneIsNotRecoveryAndRequiresNewExactRuntime(t *testing.T) {
	p, target := resumeProviderFixture(t)
	var launchArgs []string
	p.Start = func(_ string, args []string) error { launchArgs = args; return nil }
	p.Read = func() (*attention.Snapshot, error) { return &attention.Snapshot{}, nil }
	if _, err := p.Resume(target); err == nil {
		t.Fatal("terminal launch alone reported recovery")
	}
	if !reflect.DeepEqual(launchArgs, []string{"run", filepath.Join(p.Home, "Applications/AgentBell.app/Contents/MacOS/agentbell"), "resume-launch", p.Session.ID}) {
		t.Fatal("launcher included shell input or untrusted arguments", launchArgs)
	}
	started := time.Now()
	running := p.Session
	running.RuntimeState = attention.RuntimeRunning
	running.RuntimeInstanceID = "new"
	running.RuntimeStartedAt = &started
	running.ReturnTarget = &surface.ReturnTarget{Surface: "tabby", ContextID: "new-tab", Capability: surface.ReturnApp}
	p.Read = func() (*attention.Snapshot, error) {
		return &attention.Snapshot{Working: []attention.Session{running}}, nil
	}
	p.Probe = func(surface.ReturnTarget) error { return nil }
	p.Alive = func(attention.Session) bool { return true }
	if _, err := p.Resume(target); err == nil {
		t.Fatal("app-only target counted as exact recovery")
	}
	running.ReturnTarget.Capability = surface.ReturnExactContext
	p.Probe = func(surface.ReturnTarget) error { return errors.New("tab disappeared") }
	if _, err := p.Resume(target); err == nil {
		t.Fatal("missing exact tab counted as recovery")
	}
	p.Probe = func(surface.ReturnTarget) error { return nil }
	if _, err := p.Resume(target); err != nil {
		t.Fatal("new original runtime with verified tab rejected", err)
	}
	p.Alive = func(attention.Session) bool { return false }
	if _, err := p.Resume(target); err == nil {
		t.Fatal("exited runtime counted as recovery")
	}
	p.Alive = func(attention.Session) bool { return true }
	running.NativeSessionID = "12345678-1234-1234-1234-123456789abd"
	if _, err := p.Resume(target); err == nil {
		t.Fatal("different conversation counted as recovery")
	}
}

func TestTabbyRecoveryRejectsChangedTargetAndDuplicateLaunch(t *testing.T) {
	p, target := resumeProviderFixture(t)
	p.Start = func(_ string, _ []string) error { t.Fatal("changed target launched"); return nil }
	changed := target
	changed.Argv = []string{"--resume", "other"}
	if _, err := p.Resume(changed); err == nil {
		t.Fatal("mutated argv accepted")
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	p.Start = func(_ string, _ []string) error { close(entered); <-release; return errors.New("cancelled") }
	done := make(chan error, 1)
	go func() { _, err := p.Resume(target); done <- err }()
	<-entered
	second := p
	second.Start = func(_ string, _ []string) error { t.Fatal("duplicate launched"); return nil }
	if _, err := second.Resume(target); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatal("duplicate not rejected", err)
	}
	close(release)
	<-done
}
