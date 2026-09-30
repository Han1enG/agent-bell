package adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/han1eng/agent-bell/internal/event"
)

// Parse converts a hook payload into AgentBell's stable event model. It keeps
// the original payload in Raw so adapters can become stricter as schemas settle.
func Parse(source string, payload []byte) (event.AgentEvent, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return event.AgentEvent{}, fmt.Errorf("invalid JSON payload: %w", err)
	}

	e := event.AgentEvent{
		Source:    source,
		Type:      event.Type(firstString(raw, "hook_event_name", "type", "event", "event_type")),
		SessionID: firstString(raw, "session_id", "sessionId"),
		CWD:       firstString(raw, "cwd", "working_directory", "workingDirectory"),
		Project:   firstString(raw, "project", "project_name", "projectName"),
		Title:     firstString(raw, "title"),
		Message:   firstString(raw, "last_assistant_message", "message", "notification_message", "reason", "error", "error_message"),
		Raw:       append([]byte(nil), payload...),
	}
	e.Type = normalizeTypeForHook(source, string(e.Type), raw)
	e.Normalize()
	if err := e.Validate(); err != nil {
		return event.AgentEvent{}, err
	}
	return e, nil
}

func normalizeTypeForHook(source, hookType string, raw map[string]any) event.Type {
	if source == "claude" && hookType == "Notification" {
		return normalizeType(firstString(raw, "notification_type", "type"))
	}
	if source == "codex" {
		switch strings.ToLower(strings.TrimSpace(hookType)) {
		case "stop":
			return event.Done
		case "permissionrequest":
			return event.NeedsApproval
		default:
			return event.Type(strings.ToLower(strings.TrimSpace(hookType)))
		}
	}
	switch strings.ToLower(strings.TrimSpace(hookType)) {
	case "stop", "taskcompleted":
		return event.Done
	case "stopfailure", "posttoolusefailure":
		return event.Error
	case "permissionrequest", "permission_prompt":
		return event.NeedsApproval
	case "elicitation", "agent_needs_input", "input":
		return event.NeedsInput
	default:
		return normalizeType(hookType)
	}
}

func normalizeType(value string) event.Type {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "done", "completed", "complete", "success", "stop":
		return event.Done
	case "needs_approval", "approval", "permission", "permission_prompt", "permission_required", "awaiting_permission":
		return event.NeedsApproval
	case "needs_input", "input", "question", "agent_needs_input", "awaiting_input":
		return event.NeedsInput
	case "agent_completed":
		return event.Done
	case "error", "failed", "failure":
		return event.Error
	default:
		return event.Type(v)
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
