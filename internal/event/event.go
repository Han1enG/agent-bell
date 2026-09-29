package event

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Type string

const (
	Done          Type = "done"
	NeedsApproval Type = "needs_approval"
	NeedsInput    Type = "needs_input"
	Error         Type = "error"
)

func (t Type) Valid() bool {
	return t == Done || t == NeedsApproval || t == NeedsInput || t == Error
}

type AgentEvent struct {
	Source    string
	Type      Type
	SessionID string
	CWD       string
	Project   string
	Title     string
	Message   string
	Timestamp time.Time
	Raw       json.RawMessage
}

func (e AgentEvent) Validate() error {
	if e.Source != "claude" && e.Source != "codex" {
		return fmt.Errorf("unsupported source %q: expected claude or codex", e.Source)
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
