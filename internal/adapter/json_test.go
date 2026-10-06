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
		{"codex", "permission-request.json", event.EventPermissionRequest, "sess_abc123", ""},
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

func TestRealCodexAutoReviewIsRequestNotApproval(t *testing.T) {
	payload, err := os.ReadFile("../../testdata/codex/approval-auto-review.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse("codex", payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != event.EventPermissionRequest {
		t.Fatalf("pre-decision request mislabeled: %s", got.Type)
	}
}

func TestAttentionLifecycleHookClassification(t *testing.T) {
	for _, source := range []string{"claude", "codex"} {
		for _, tc := range []struct {
			name string
			want event.Type
		}{{"SessionStart", event.SessionStarted}, {"UserPromptSubmit", event.Working}, {"PreToolUse", event.ToolActivity}, {"PostToolUse", event.ToolActivity}} {
			e, err := Parse(source, []byte(`{"hook_event_name":"`+tc.name+`","session_id":"a","prompt":"never retained"}`))
			if err != nil || e.Type != tc.want {
				t.Fatalf("%s %s: %+v %v", source, tc.name, e, err)
			}
		}
	}
	for _, kind := range []string{"agent_needs_input", "elicitation_dialog"} {
		e, err := Parse("claude", []byte(`{"hook_event_name":"Notification","notification_type":"`+kind+`"}`))
		if err != nil || e.Type != event.NeedsInput {
			t.Fatal(kind, e, err)
		}
	}
	e, err := Parse("claude", []byte(`{"hook_event_name":"PermissionRequest"}`))
	if err != nil || e.AttentionConfirmed {
		t.Fatal("pre-routing approval confirmed", e, err)
	}
	e, err = Parse("claude", []byte(`{"hook_event_name":"Notification","notification_type":"permission_prompt"}`))
	if err != nil || !e.AttentionConfirmed || e.Type != event.NeedsApproval {
		t.Fatal("reliable approval missing", e, err)
	}
}

func TestIdlePromptIsNotHumanInputWait(t *testing.T) {
	idle, err := Parse("claude", []byte(`{"hook_event_name":"Notification","notification_type":"idle_prompt","session_id":"idle","message":"Claude is waiting for your input"}`))
	if err != nil || idle.Type != event.Idle {
		t.Fatalf("idle misclassified: %+v %v", idle, err)
	}
	for _, kind := range []string{"agent_needs_input", "elicitation_dialog", "elicitation_url_dialog"} {
		got, err := Parse("claude", []byte(`{"hook_event_name":"Notification","notification_type":"`+kind+`","session_id":"blocked"}`))
		if err != nil || got.Type != event.NeedsInput {
			t.Fatalf("real wait lost: %s %+v %v", kind, got, err)
		}
	}
}

func TestClaudeSessionEndReasonsAndCodexDoesNotInventHook(t *testing.T) {
	for _, reason := range []string{"clear", "resume", "logout", "prompt_input_exit", "other"} {
		got, err := Parse("claude", []byte(`{"hook_event_name":"SessionEnd","session_id":"s","reason":"`+reason+`"}`))
		if err != nil || got.Type != event.SessionEnded || got.ExitReason != reason {
			t.Fatal(got, err)
		}
	}
	if _, err := Parse("codex", []byte(`{"hook_event_name":"SessionEnd","session_id":"s"}`)); err == nil {
		t.Fatal("unverified Codex exit hook accepted")
	}
}
