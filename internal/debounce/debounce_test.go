package debounce

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
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
