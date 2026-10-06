package main

import (
	"encoding/json"
	"errors"
	"net"
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
	s := attention.Session{ID: "claude:12345678-1234-1234-1234-123456789abc", Agent: "claude", NativeSessionID: "12345678-1234-1234-1234-123456789abc", AgentFlavor: "claude_cli", RuntimeState: attention.RuntimeExited, RuntimeInstanceID: "old", CWD: home, ReturnTarget: &surface.ReturnTarget{Surface: "tabby"}}
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
	if !reflect.DeepEqual(launchArgs, []string{"resume-launch", p.Session.ID}) {
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

func TestRecoveryNeverSwitchesAnotherTerminalToTabby(t *testing.T) {
	p, target := resumeProviderFixture(t)
	p.Session.ReturnTarget.Surface = "terminal"
	p.Start = func(string, []string) error { t.Fatal("other terminal launched in Tabby"); return nil }
	if _, err := p.Resume(target); err == nil {
		t.Fatal("original terminal was ignored")
	}
}

func recoveryBridgeFixture(t *testing.T, home, window string, available bool, launched chan<- string) {
	t.Helper()
	dir := filepath.Join(home, ".cache", "agentbell", "tabby")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(dir, window+".sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			var request map[string]string
			json.NewDecoder(conn).Decode(&request)
			var response any
			switch request["operation"] {
			case "list":
				response = map[string]any{"contexts": []any{}}
			case "capabilities":
				response = map[string]any{"resume": available}
			case "resume":
				launched <- window
				response = map[string]any{"ok": true}
			}
			json.NewEncoder(conn).Encode(response)
			conn.Close()
		}
	}()
}
func TestTabbyRecoveryUsesOriginalWindowAndOldPluginCreatesNothing(t *testing.T) {
	p, _ := resumeProviderFixture(t)
	p.Home, _ = os.MkdirTemp("/tmp", "abr4-")
	t.Cleanup(func() { os.RemoveAll(p.Home) })
	first := "12345678-1234-1234-1234-123456789abc"
	original := "22345678-1234-1234-1234-123456789abc"
	launched := make(chan string, 4)
	recoveryBridgeFixture(t, p.Home, first, true, launched)
	recoveryBridgeFixture(t, p.Home, original, true, launched)
	p.Session.ReturnTarget.ContextID = original + ":tab"
	if err := p.openTab("unused"); err != nil {
		t.Fatal(err)
	}
	if got := <-launched; got != original {
		t.Fatal("did not reuse original window", got)
	}
	old, _ := resumeProviderFixture(t)
	old.Home, _ = os.MkdirTemp("/tmp", "abr4-")
	t.Cleanup(func() { os.RemoveAll(old.Home) })
	recoveryBridgeFixture(t, old.Home, first, false, launched)
	if err := old.openTab("unused"); err == nil || !strings.Contains(err.Error(), "restart") {
		t.Fatal("old plugin not rejected", err)
	}
	select {
	case <-launched:
		t.Fatal("old plugin created a tab")
	default:
	}
}

func TestTabbyRecoveryActivatesWindowlessAppAndWaitsForBridge(t *testing.T) {
	p, _ := resumeProviderFixture(t)
	p.Home, _ = os.MkdirTemp("/tmp", "abr5-")
	t.Cleanup(func() { os.RemoveAll(p.Home) })
	launched := make(chan string, 1)
	window := "12345678-1234-1234-1234-123456789abc"
	calls := 0
	p.OpenApplication = func(app string) error {
		calls++
		if app != "/Applications/Tabby.app" {
			t.Fatal("unexpected original app", app)
		}
		recoveryBridgeFixture(t, p.Home, window, true, launched)
		return nil
	}
	if err := p.openTab("/Applications/Tabby.app/Contents/MacOS/Tabby"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || <-launched != window {
		t.Fatal("windowless App activation failed")
	}
	p.OpenApplication = func(string) error { t.Fatal("existing window reactivated or replaced"); return nil }
	if err := p.openTab("unused"); err != nil {
		t.Fatal(err)
	}
	if <-launched != window {
		t.Fatal("existing window not reused")
	}
}
