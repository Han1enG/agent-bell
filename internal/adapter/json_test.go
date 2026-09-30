package adapter

import (
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
