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

// Keep exact dismissal watermarks instead of evicting them and admitting ghosts.
// At capacity, new identities use the existing notification fallback; existing
// sessions, explicit restart of their identities and Dismiss keep working.
const MaxTrackedSessions = 10000
const MaxTombstoneJSONBytes = 16 * 1024

var ErrTrackingCapacity = errors.New("session tracking capacity reached; dismissal watermarks retained")

type SessionStatus string

const (
	SessionClosed   SessionStatus = "closed"
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

type RuntimeState string

const (
	RuntimeRunning RuntimeState = "running"
	RuntimeExited  RuntimeState = "exited"
	RuntimeUnknown RuntimeState = "unknown"
)

type ProbeResult string

const (
	Alive   ProbeResult = "alive"
	Exited  ProbeResult = "exited"
	Unknown ProbeResult = "unknown"
)

type Session struct {
	SurfaceState              string       `json:"surface_state,omitempty"`
	LastExitReason            string       `json:"last_exit_reason,omitempty"`
	PreviousRuntimeInstanceID string       `json:"previous_runtime_instance_id,omitempty"`
	ConcurrentRuntime         bool         `json:"concurrent_runtime,omitempty"`
	Action                    string       `json:"action,omitempty"`
	ActionReason              string       `json:"action_reason,omitempty"`
	NativeSessionID           string       `json:"native_session_id,omitempty"`
	AgentFlavor               string       `json:"agent_flavor,omitempty"`
	RuntimeState              RuntimeState `json:"runtime_state,omitempty"`
	RuntimeInstanceID         string       `json:"runtime_instance_id,omitempty"`
	RuntimeStartedAt          *time.Time   `json:"runtime_started_at,omitempty"`
	ExitedAt                  *time.Time   `json:"exited_at,omitempty"`
	ExitReason                string       `json:"exit_reason,omitempty"`
	RecoveryCapability        string       `json:"recovery_capability,omitempty"`
	DismissedAt               *time.Time   `json:"dismissed_at,omitempty"`

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
	TrackingWarning string    `json:"tracking_warning,omitempty"`
	ObservedAgents  []string  `json:"observed_agents,omitempty"`
	RecentLimit     int       `json:"recent_limit,omitempty"`
	SchemaVersion   int       `json:"schema_version"`
	NeedsYou        []Session `json:"needs_you"`
	Working         []Session `json:"working"`
	Closed          []Session `json:"closed,omitempty"`
	Recent          []Session `json:"recent"`
	Paused          bool      `json:"paused"`
	AppVersion      string    `json:"app_version,omitempty"`
	StorageError    string    `json:"storage_error,omitempty"`
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
	if exists && e.ProcessIdentity != "" && s.ProcessIdentity != "" && (e.ProcessIdentity != s.ProcessIdentity || e.ProcessID != s.ProcessID) {
		instanceKey := fmt.Sprintf("%s:runtime:%x", id, sha256.Sum256([]byte(fmt.Sprintf("%d:%s", e.ProcessID, e.ProcessIdentity))))
		if other, ok := m.Sessions[instanceKey]; ok {
			id, s = instanceKey, other
		} else if e.Type == event.SessionStarted && s.RuntimeState != RuntimeExited && s.DismissedAt == nil {
			id, exists = instanceKey, false
		} else if e.Type != event.SessionStarted {
			return false, nil
		}
	}
	if exists && e.Type == event.SessionStarted && s.RuntimeState != RuntimeExited && s.RuntimeStartedAt != nil && e.ProcessID == s.ProcessID && e.ProcessIdentity == s.ProcessIdentity {
		return false, nil
	}
	if exists && s.DismissedAt != nil {
		if e.Type != event.SessionEnded && (!e.Timestamp.After(*s.DismissedAt) || e.Type != event.SessionStarted) {
			return false, nil
		}
		if e.Type == event.SessionStarted {
			s.DismissedAt = nil
		}
	}
	if exists && s.RuntimeState == RuntimeExited && e.Type != event.SessionStarted {
		return false, nil
	}
	if exists && e.Timestamp.Before(s.UpdatedAt) {
		return false, nil
	}
	if e.Type == event.ToolActivity && !exists {
		return false, nil
	}
	if !exists {
		if len(m.Sessions) >= MaxTrackedSessions {
			return false, ErrTrackingCapacity
		}
		s = Session{ID: id, Agent: e.Source, StartedAt: e.Timestamp, Attention: AttentionNone, Status: SessionUnknown}
	}
	oldStatus, oldAttention := s.Status, s.Attention
	if e.SessionID != "" {
		s.NativeSessionID = e.SessionID
	}
	if e.Type == event.SessionStarted {
		s.AgentFlavor = "unknown"
	}
	if e.AgentFlavor != "" {
		s.AgentFlavor = e.AgentFlavor
	}
	if s.AgentFlavor == "" {
		s.AgentFlavor = "unknown"
	}
	if e.Type == event.SessionStarted {
		s.PreviousRuntimeInstanceID = s.RuntimeInstanceID
		s.SurfaceState = "unknown"
		s.RuntimeState = RuntimeRunning
		s.ExitedAt = nil
		s.ExitReason = ""
		s.RuntimeStartedAt = &e.Timestamp
		s.RuntimeInstanceID = fmt.Sprintf("%d:%s:%d", e.ProcessID, e.ProcessIdentity, e.Timestamp.UnixNano())
		s.ProcessID, s.ProcessIdentity = e.ProcessID, e.ProcessIdentity
		s.ReturnTarget = nil
		s.AttentionAt = nil
	} else if s.RuntimeState == "" {
		s.RuntimeState = RuntimeUnknown
	}
	s.RecoveryCapability = "unknown"
	if (s.AgentFlavor == "claude_cli" || s.AgentFlavor == "codex_cli") && validNativeID(s.NativeSessionID) {
		s.RecoveryCapability = "supported"
	}
	if s.AgentFlavor == "codex_desktop" {
		s.RecoveryCapability = "unsupported"
	}
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
	case event.SessionEnded:
		closeRuntime(&s, e.Timestamp, e.ExitReason)
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
	if e.Type != event.ToolActivity && e.Type != event.Working && e.Type != event.SessionStarted && e.Type != event.SessionEnded {
		s.Summary = notify.Summary(e.Message)
	}
	m.Sessions[id] = s
	return !exists || oldStatus != s.Status || oldAttention != s.Attention, nil
}
func (m *Engine) Snapshot() Snapshot {
	v := Snapshot{SchemaVersion: 1, NeedsYou: []Session{}, Working: []Session{}, Recent: []Session{}, Paused: m.Paused}
	if len(m.Sessions) >= MaxTrackedSessions {
		v.TrackingWarning = ErrTrackingCapacity.Error()
	}
	for _, s := range m.Sessions {
		if s.DismissedAt != nil {
			continue
		}
		if s.RuntimeState == RuntimeExited && s.NativeSessionID != "" {
			for id, other := range m.Sessions {
				if id != s.ID && other.Agent == s.Agent && other.NativeSessionID == s.NativeSessionID && other.RuntimeState != RuntimeExited {
					s.ConcurrentRuntime = true
				}
			}
		}
		s.Action, s.ActionReason = SessionAction(s)
		if s.Status == SessionClosed {
			v.Closed = append(v.Closed, s)
		} else if s.Attention != AttentionNone {
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
	sort.Slice(v.Closed, func(i, j int) bool {
		a, b := v.Closed[i], v.Closed[j]
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.ID < b.ID
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	if len(v.Closed) > 5 {
		v.Closed = v.Closed[:5]
	}
	return v
}
func (m *Engine) Cleanup(now time.Time) {
	for id, s := range m.Sessions {
		// After normal retention, keep only the watermark and runtime/native identity.
		// This small tombstone must outlive history retention to reject delayed hooks.
		if s.DismissedAt != nil && s.DismissedAt.Before(now.AddDate(0, 0, -m.RetentionDays)) {
			// Only identity, time watermarks and process-generation evidence survive.
			m.Sessions[id] = Session{ID: s.ID, Agent: s.Agent, NativeSessionID: s.NativeSessionID,
				Status: s.Status, Attention: AttentionNone, RuntimeState: s.RuntimeState,
				RuntimeInstanceID: s.RuntimeInstanceID, RuntimeStartedAt: s.RuntimeStartedAt,
				ProcessID: s.ProcessID, ProcessIdentity: s.ProcessIdentity,
				StartedAt: s.StartedAt, UpdatedAt: s.UpdatedAt, DismissedAt: s.DismissedAt,
				LastEventType: "dismissed"}
			continue
		}
		if (s.Status == SessionDone || s.Status == SessionUnknown || s.Status == SessionClosed) && s.DismissedAt == nil && s.UpdatedAt.Before(now.AddDate(0, 0, -m.RetentionDays)) {
			delete(m.Sessions, id)
		}
	}
}
func (m *Engine) ClearRecent() {
	for id, s := range m.Sessions {
		if s.Status == SessionDone {
			_ = m.Dismiss(id, time.Now())
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
	return m.Dismiss(id, time.Now())
}

// Dismiss retains an event watermark in the persisted session. It never touches an agent.
func (m *Engine) Dismiss(id string, now time.Time) error {
	s, ok := m.Sessions[id]
	if !ok {
		return nil
	}
	if s.DismissedAt != nil {
		return nil
	}
	if now.Before(s.UpdatedAt) {
		now = s.UpdatedAt
	}
	s.DismissedAt = &now
	m.Sessions[id] = s
	return nil
}
func (m *Engine) ClearAll(now time.Time) {
	for id := range m.Sessions {
		_ = m.Dismiss(id, now)
	}
}
func closeRuntime(s *Session, now time.Time, reason string) {
	s.Status = SessionClosed
	s.RuntimeState = RuntimeExited
	s.ExitedAt = &now
	s.ExitReason = reason
	s.LastExitReason = reason
	s.UpdatedAt = now
	s.Attention = AttentionNone
	s.AttentionAt = nil
}

// Probe only runtime identity. Surface availability is a separate Return concern.
func (m *Engine) Reconcile(now time.Time, probe func(Session) ProbeResult) {
	for id, s := range m.Sessions {
		if s.RuntimeState == RuntimeExited || s.DismissedAt != nil {
			continue
		}
		switch probe(s) {
		case Exited:
			closeRuntime(&s, now, "process_exited")
		case Alive:
			s.RuntimeState = RuntimeRunning
		default:
			s.RuntimeState = RuntimeUnknown
		}
		m.Sessions[id] = s
	}
}
