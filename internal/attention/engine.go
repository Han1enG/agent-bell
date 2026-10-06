// Package attention owns session state; notifications never control tracking.
package attention

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/notify"
	"github.com/han1eng/agent-bell/internal/surface"
)

type SessionStatus string

const (
	SessionWorking  SessionStatus = "working"
	SessionNeedsYou SessionStatus = "needs_you"
	SessionDone     SessionStatus = "done"
	SessionError    SessionStatus = "error"
	SessionUnknown  SessionStatus = "unknown"
)

type AttentionState string

const (
	AttentionNone     AttentionState = "none"
	AttentionInput    AttentionState = "input"
	AttentionApproval AttentionState = "approval"
	AttentionError    AttentionState = "error"
)

type Session struct {
	Title           string                `json:"title,omitempty"`
	ID              string                `json:"id"`
	Agent           string                `json:"agent"`
	Project         string                `json:"project"`
	CWD             string                `json:"cwd"`
	Status          SessionStatus         `json:"status"`
	Attention       AttentionState        `json:"attention"`
	Summary         string                `json:"summary"`
	ReturnTarget    *surface.ReturnTarget `json:"return_target,omitempty"`
	WorkingAt       *time.Time            `json:"working_at,omitempty"`
	StartedAt       time.Time             `json:"started_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	FinishedAt      *time.Time            `json:"finished_at,omitempty"`
	AttentionAt     *time.Time            `json:"attention_at,omitempty"`
	LastEventType   string                `json:"last_event_type"`
	ProcessID       int                   `json:"process_id,omitempty"`
	ProcessIdentity string                `json:"process_identity,omitempty"`
}
type Snapshot struct {
	ObservedAgents []string  `json:"observed_agents,omitempty"`
	RecentLimit    int       `json:"recent_limit,omitempty"`
	SchemaVersion  int       `json:"schema_version"`
	NeedsYou       []Session `json:"needs_you"`
	Working        []Session `json:"working"`
	Recent         []Session `json:"recent"`
	Paused         bool      `json:"paused"`
	AppVersion     string    `json:"app_version,omitempty"`
	StorageError   string    `json:"storage_error,omitempty"`
}
type Engine struct {
	Sessions      map[string]Session
	Paused        bool
	RetentionDays int
}

func New(days int) *Engine {
	if days < 1 {
		days = 7
	}
	return &Engine{Sessions: map[string]Session{}, RetentionDays: days}
}
func SessionKey(e event.AgentEvent) (string, error) {
	if e.SessionID != "" {
		return e.Source + ":" + e.SessionID, nil
	}
	if e.ProcessID <= 1 || e.ProcessIdentity == "" {
		return "", errors.New("event has no reliable session or process identity")
	}
	context := ""
	if e.ReturnTarget != nil {
		context = e.ReturnTarget.Surface + ":" + e.ReturnTarget.ContextID
	}
	return fmt.Sprintf("%s:temporary:%x", e.Source, sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s", e.ProcessID, e.ProcessIdentity, context)))), nil
}

// Apply returns true only for a meaningful transition to persist in minimal events.
func (m *Engine) Apply(e event.AgentEvent) (bool, error) {
	if err := e.Validate(); err != nil {
		return false, err
	}
	e.Normalize()
	// A completed response waiting for the next prompt is not unresolved work.
	// Stop owns completion time and summary; idle must not recreate cleared history.
	if e.Type == event.Idle {
		return false, nil
	}
	// Pre-routing permission signals cannot create or modify attention.
	if e.Type == event.EventPermissionRequest || e.Type == event.NeedsApproval && !e.AttentionConfirmed {
		return false, nil
	}
	id, err := SessionKey(e)
	if err != nil {
		return false, err
	}
	s, exists := m.Sessions[id]
	if exists && e.Timestamp.Before(s.UpdatedAt) {
		return false, nil
	}
	if e.Type == event.ToolActivity && !exists {
		return false, nil
	}
	if !exists {
		s = Session{ID: id, Agent: e.Source, StartedAt: e.Timestamp, Attention: AttentionNone, Status: SessionUnknown}
	}
	oldStatus, oldAttention := s.Status, s.Attention
	if title := CleanTitle(e.SessionTitle); title != "" {
		s.Title = title
	}
	s.UpdatedAt = e.Timestamp
	s.LastEventType = string(e.Type)
	if e.Project != "" {
		s.Project = e.Project
	}
	if e.CWD != "" {
		s.CWD = e.CWD
	}
	if e.ProcessID > 1 && e.ProcessIdentity != "" {
		s.ProcessID = e.ProcessID
		s.ProcessIdentity = e.ProcessIdentity
	}
	if e.ReturnTarget != nil {
		b, _ := json.Marshal(e.ReturnTarget)
		var t surface.ReturnTarget
		_ = json.Unmarshal(b, &t)
		s.ReturnTarget = &t
	}
	switch e.Type {
	case event.SessionStarted, event.Working:
		if e.Type == event.Working || oldStatus != SessionWorking || s.WorkingAt == nil {
			t := e.Timestamp
			s.WorkingAt = &t
		}
		s.Status = SessionWorking
		s.Attention = AttentionNone
		s.FinishedAt = nil
		s.Summary = ""
	case event.ToolActivity:
		// Activity alone refreshes metadata; it cannot resolve a human wait.
	case event.NeedsInput:
		s.Status = SessionNeedsYou
		s.Attention = AttentionInput
		s.FinishedAt = nil
	case event.NeedsApproval:
		s.Status = SessionNeedsYou
		s.Attention = AttentionApproval
		s.FinishedAt = nil
	case event.Done:
		s.Status = SessionDone
		s.Attention = AttentionNone
		s.FinishedAt = &s.UpdatedAt
	case event.Error:
		s.Status = SessionError
		s.Attention = AttentionError
		s.FinishedAt = &s.UpdatedAt
	}
	if s.Attention == AttentionNone {
		s.AttentionAt = nil
	} else if oldAttention != s.Attention || s.AttentionAt == nil {
		t := s.UpdatedAt
		s.AttentionAt = &t
	}
	if e.Type != event.ToolActivity && e.Type != event.Working && e.Type != event.SessionStarted {
		s.Summary = notify.Summary(e.Message)
	}
	m.Sessions[id] = s
	return !exists || oldStatus != s.Status || oldAttention != s.Attention, nil
}
func (m *Engine) Snapshot() Snapshot {
	v := Snapshot{SchemaVersion: 1, NeedsYou: []Session{}, Working: []Session{}, Recent: []Session{}, Paused: m.Paused}
	for _, s := range m.Sessions {
		if s.Attention != AttentionNone {
			v.NeedsYou = append(v.NeedsYou, s)
		} else if s.Status == SessionWorking {
			v.Working = append(v.Working, s)
		} else if s.Status == SessionDone {
			v.Recent = append(v.Recent, s)
		}
	}
	sort.Slice(v.NeedsYou, func(i, j int) bool {
		a, b := v.NeedsYou[i], v.NeedsYou[j]
		ta, tb := a.UpdatedAt, b.UpdatedAt
		if a.AttentionAt != nil {
			ta = *a.AttentionAt
		}
		if b.AttentionAt != nil {
			tb = *b.AttentionAt
		}
		if ta.Equal(tb) {
			return a.ID < b.ID
		}
		return ta.Before(tb)
	})
	sort.Slice(v.Working, func(i, j int) bool {
		a, b := v.Working[i], v.Working[j]
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.ID < b.ID
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	sort.Slice(v.Recent, func(i, j int) bool {
		a, b := v.Recent[i], v.Recent[j]
		if a.FinishedAt == nil || b.FinishedAt == nil {
			return a.ID < b.ID
		}
		if a.FinishedAt.Equal(*b.FinishedAt) {
			return a.ID < b.ID
		}
		return a.FinishedAt.After(*b.FinishedAt)
	})
	return v
}
func (m *Engine) Cleanup(now time.Time) {
	for id, s := range m.Sessions {
		if (s.Status == SessionDone || s.Status == SessionUnknown) && s.UpdatedAt.Before(now.AddDate(0, 0, -m.RetentionDays)) {
			delete(m.Sessions, id)
		}
	}
}
func (m *Engine) ClearRecent() {
	for id, s := range m.Sessions {
		if s.Status == SessionDone {
			delete(m.Sessions, id)
		}
	}
}

// RemoveRecent cannot delete an active or unresolved attention session.
func (m *Engine) RemoveRecent(id string) error {
	s, ok := m.Sessions[id]
	if !ok {
		return nil
	}
	if s.Status != SessionDone || s.Attention != AttentionNone {
		return errors.New("only recent completed sessions can be removed")
	}
	delete(m.Sessions, id)
	return nil
}

// A negative lifecycle check archives stale state without inventing completion.
// Missing identity ages out after 24h; provider failures alone are inconclusive.
func (m *Engine) Reconcile(now time.Time, alive func(Session) bool) {
	for id, s := range m.Sessions {
		if s.Status != SessionWorking && s.Status != SessionNeedsYou && s.Status != SessionError {
			continue
		}
		stale := !alive(s) || (s.ProcessIdentity == "" && now.Sub(s.UpdatedAt) > 24*time.Hour)
		if stale {
			s.Status = SessionUnknown
			s.Attention = AttentionNone
			s.AttentionAt = nil
			s.FinishedAt = nil
			s.LastEventType = "stale"
			m.Sessions[id] = s
		}
	}
}
