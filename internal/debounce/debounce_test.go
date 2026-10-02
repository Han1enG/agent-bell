package debounce

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/surface"
)

func TestSuppressesSameEventForThreeSeconds(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	e := event.AgentEvent{Source: "claude", Type: event.Done, SessionID: "s1", CWD: "/tmp/demo"}
	if Suppressed(home, e, now) {
		t.Fatal("first event suppressed")
	}
	if !Suppressed(home, e, now.Add(2*time.Second)) {
		t.Fatal("duplicate event not suppressed")
	}
	if Suppressed(home, e, now.Add(4*time.Second)) {
		t.Fatal("event outside window suppressed")
	}
}

func TestFallsBackToCWDWhenSessionMissing(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	e := event.AgentEvent{Source: "codex", Type: event.NeedsApproval, CWD: "/tmp/demo"}
	if Suppressed(home, e, now) || !Suppressed(home, e, now.Add(time.Second)) {
		t.Fatal("cwd-based key was not applied")
	}
}

func TestCacheFailureDoesNotSuppressNotification(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "Library"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := event.AgentEvent{Source: "claude", Type: event.Done, SessionID: "s1"}
	if Suppressed(home, e, time.Now()) {
		t.Fatal("cache failure must fail open")
	}
}

func TestDistinctSurfaceContextsWithSameCWD(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	a := event.AgentEvent{Source: "codex", Type: event.Done, CWD: "/same", ReturnTarget: &surface.ReturnTarget{Surface: "tabby", ContextID: "A"}}
	b := a
	b.ReturnTarget = &surface.ReturnTarget{Surface: "tabby", ContextID: "C"}
	if Suppressed(home, a, now) || Suppressed(home, b, now) {
		t.Fatal("different contexts were suppressed")
	}
	if !Suppressed(home, a, now.Add(time.Second)) {
		t.Fatal("same context not debounced")
	}
}
