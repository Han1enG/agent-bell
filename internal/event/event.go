package event

import (
	"encoding/json"
	"fmt"
	"github.com/han1eng/agent-bell/internal/surface"
	"path/filepath"
	"strings"
	"time"
)

type Type string

const (
	Idle                   Type = "idle_prompt"
	SessionStarted         Type = "session_started"
	Working                Type = "working"
	ToolActivity           Type = "tool_activity"
	EventPermissionRequest Type = "permission_request"
	Done                   Type = "done"
	NeedsApproval          Type = "needs_approval"
	NeedsInput             Type = "needs_input"
	Error                  Type = "error"
)

func (t Type) Valid() bool {
	return t == Idle || t == SessionStarted || t == Working || t == ToolActivity || t == EventPermissionRequest || t == Done || t == NeedsApproval || t == NeedsInput || t == Error
}

type AgentEvent struct {
	ReturnTarget       *surface.ReturnTarget
	Source             string
	Type               Type
	SessionID          string
	CWD                string
	Project            string
	Title              string
	Message            string
	Timestamp          time.Time
	Raw                json.RawMessage `json:"-"`
	ProcessID          int
	ProcessIdentity    string
	AttentionConfirmed bool
}

func (e AgentEvent) Validate() error {
	if e.Source != "claude" && e.Source != "codex" {
		return fmt.Errorf("unsupported source %q: expected claude or codex", e.Source)
	}
	if len(e.SessionID) > 512 || len(e.CWD) > 4096 || len(e.Project) > 512 || len(e.ProcessIdentity) > 256 {
		return fmt.Errorf("event metadata exceeds size limit")
	}
	if e.ProcessID < 0 {
		return fmt.Errorf("invalid process ID")
	}
	if !e.Type.Valid() {
		return fmt.Errorf("unsupported event type %q", e.Type)
	}
	return nil
}

func (e *AgentEvent) Normalize() {
	e.Source = strings.ToLower(strings.TrimSpace(e.Source))
	e.Project = strings.TrimSpace(e.Project)
	if e.Project == "" && e.CWD != "" {
		e.Project = filepath.Base(filepath.Clean(e.CWD))
	}
	if e.Project == "" {
		e.Project = strings.Title(e.Source)
	}
	if e.Title == "" {
		e.Title = displayName(e.Source)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
}

func displayName(source string) string {
	if source == "claude" {
		return "Claude Code"
	}
	if source == "codex" {
		return "Codex"
	}
	return "AgentBell"
}
