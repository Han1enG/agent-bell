package adapter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/han1eng/agent-bell/internal/event"
)

func TestParseNormalizesCommonPayload(t *testing.T) {
	payload := []byte(`{"event":"completed","session_id":"s-1","cwd":"/tmp/agent-teams","message":"finished"}`)
	got, err := Parse("claude", payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != event.Done || got.Project != "agent-teams" || got.SessionID != "s-1" {
		t.Fatalf("unexpected event: %+v", got)
	}
}

func TestOfficialHookFixtures(t *testing.T) {
	tests := []struct {
		source, file     string
		want             event.Type
		session, message string
	}{
		{"claude", "stop.json", event.Done, "abc123", "I've completed the refactoring."},
		{"claude", "permission-request.json", event.NeedsApproval, "abc123", ""},
		{"claude", "notification.json", event.NeedsApproval, "abc123", "Claude needs your permission"},
		{"claude", "stop-failure.json", event.Error, "abc123", "API Error: Rate limit reached"},
		{"codex", "stop.json", event.Done, "sess_abc123", "Implemented the change and verified the tests."},
		{"codex", "permission-request.json", event.NeedsApproval, "sess_abc123", ""},
	}
	for _, tt := range tests {
		t.Run(tt.source+"/"+tt.file, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join("..", "..", "testdata", tt.source, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(tt.source, payload)
			if err != nil {
				t.Fatal(err)
			}
			if got.Type != tt.want || got.CWD != "/Users/example/Code/demo" || got.SessionID != tt.session || got.Message != tt.message || got.Project != "demo" {
				t.Fatalf("unexpected event: %+v", got)
			}
		})
	}
}

func TestParseCaseInsensitiveKeysAndUnknownEvent(t *testing.T) {
	got, err := Parse("CLAUDE", []byte(`{"Hook_Event_Name":"sToP","Session_ID":"s1","CWD":"/tmp/demo"}`))
	if err != nil || got.Type != event.Done || got.SessionID != "s1" {
		t.Fatalf("case normalization failed: %+v, %v", got, err)
	}
	_, err = Parse("claude", []byte(`{"hook_event_name":"SomeFutureEvent"}`))
	if err == nil || !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("expected ignorable unknown event, got %v", err)
	}
}

func TestParseRejectsUnknownEvent(t *testing.T) {
	_, err := Parse("codex", []byte(`{"type":"unknown"}`))
	if err == nil {
		t.Fatal("expected unknown event to be rejected")
	}
}

func TestParseClaudeNotificationType(t *testing.T) {
	got, err := Parse("claude", []byte(`{"hook_event_name":"Notification","notification_type":"permission_prompt","cwd":"/tmp/demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != event.NeedsApproval {
		t.Fatalf("expected approval event, got %q", got.Type)
	}
}

func TestParseCodexStopUsesAssistantSummary(t *testing.T) {
	got, err := Parse("codex", []byte(`{"hook_event_name":"Stop","last_assistant_message":"Implemented the change.","cwd":"/tmp/demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != event.Done || got.Message != "Implemented the change." {
		t.Fatalf("unexpected event: %+v", got)
	}
}

func TestParseStopUsesAssistantSummary(t *testing.T) {
	got, err := Parse("claude", []byte(`{"hook_event_name":"Stop","last_assistant_message":"Implemented the change and verified the tests.","cwd":"/tmp/demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "Implemented the change and verified the tests." {
		t.Fatalf("expected assistant summary, got %q", got.Message)
	}
}
