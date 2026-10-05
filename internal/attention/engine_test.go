package attention

import (
	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/surface"
	"testing"
	"time"
)

func fixture(id string, typ event.Type, at time.Time) event.AgentEvent {
	return event.AgentEvent{Source: "claude", SessionID: id, Project: "same", CWD: "/same", Type: typ, Timestamp: at, AttentionConfirmed: true}
}
func apply(t *testing.T, m *Engine, e event.AgentEvent) {
	t.Helper()
	if _, err := m.Apply(e); err != nil {
		t.Fatal(err)
	}
}
func TestLifecycleAndBadge(t *testing.T) {
	now := time.Now()
	m := New(7)
	apply(t, m, fixture("a", event.SessionStarted, now))
	apply(t, m, fixture("b", event.Working, now))
	for i := 0; i < 10; i++ {
		apply(t, m, fixture("a", event.NeedsInput, now.Add(time.Duration(i)*time.Second)))
	}
	if v := m.Snapshot(); len(v.NeedsYou) != 1 || len(v.Working) != 1 || len(m.Sessions) != 2 {
		t.Fatalf("merge/badge: %+v", v)
	}
	if got := m.Sessions["claude:a"].AttentionAt; got == nil || !got.Equal(now) {
		t.Fatal("duplicate wait reset attention age")
	}
	apply(t, m, fixture("a", event.ToolActivity, now.Add(10*time.Second)))
	if len(m.Snapshot().NeedsYou) != 1 {
		t.Fatal("tool activity cleared wait")
	}
	apply(t, m, fixture("a", event.Working, now.Add(11*time.Second)))
	if len(m.Snapshot().NeedsYou) != 0 || len(m.Snapshot().Working) != 2 {
		t.Fatal("resume did not clear attention")
	}
	apply(t, m, fixture("a", event.NeedsInput, now.Add(12*time.Second)))
	apply(t, m, fixture("a", event.Done, now.Add(13*time.Second)))
	if v := m.Snapshot(); len(v.NeedsYou) != 0 || len(v.Recent) != 1 || v.Recent[0].FinishedAt == nil {
		t.Fatal("completion did not clear attention")
	}
	apply(t, m, fixture("b", event.Error, now.Add(14*time.Second)))
	if v := m.Snapshot(); len(v.NeedsYou) != 1 || v.NeedsYou[0].Attention != AttentionError {
		t.Fatal("failure not counted")
	}
	m.ClearRecent()
	if len(m.Snapshot().NeedsYou) != 1 || len(m.Snapshot().Recent) != 0 {
		t.Fatal("clear removed live error attention")
	}
}
func TestObservedPermissionDoesNotAffectAttention(t *testing.T) {
	m := New(7)
	now := time.Now()
	e := fixture("c", event.EventPermissionRequest, now)
	e.Source = "codex"
	apply(t, m, e)
	if len(m.Sessions) != 0 || len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("observed request created attention")
	}
	e.Type = event.Working
	apply(t, m, e)
	e.Type = event.EventPermissionRequest
	apply(t, m, e)
	if len(m.Snapshot().Working) != 1 || len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("request changed working state")
	}
	e = fixture("a", event.NeedsApproval, now)
	e.AttentionConfirmed = false
	apply(t, m, e)
	if len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("pre-routing Claude request created attention")
	}
	e.AttentionConfirmed = true
	apply(t, m, e)
	if len(m.Snapshot().NeedsYou) != 1 {
		t.Fatal("confirmed approval missing")
	}
}
func TestOrderingRetentionReplacementAndStale(t *testing.T) {
	now := time.Now()
	m := New(7)
	for i, id := range []string{"a", "b", "c"} {
		e := fixture(id, event.Done, now.Add(time.Duration(i)*time.Minute))
		e.ReturnTarget = &surface.ReturnTarget{Surface: "tmux", ContextID: "same-pane"}
		apply(t, m, e)
	}
	if v := m.Snapshot(); len(v.Recent) != 3 || v.Recent[0].ID != "claude:c" {
		t.Fatal("same pane merged sessions or recent order incorrect")
	}
	apply(t, m, fixture("old", event.Done, now.AddDate(0, 0, -8)))
	apply(t, m, fixture("live", event.Working, now.AddDate(0, 0, -8)))
	m.Cleanup(now)
	if _, ok := m.Sessions["claude:old"]; ok {
		t.Fatal("retention did not prune")
	}
	if _, ok := m.Sessions["claude:live"]; !ok {
		t.Fatal("retention deleted active")
	}
	e := fixture("alive", event.Working, now)
	e.ProcessID = 123
	e.ProcessIdentity = "generation"
	apply(t, m, e)
	m.Reconcile(now, func(s Session) bool { return s.ID == "claude:alive" })
	if m.Sessions["claude:live"].Status != SessionUnknown || len(m.Snapshot().Working) != 1 {
		t.Fatal("stale reconciliation fabricated done or retained stale working")
	}
	apply(t, m, fixture("wait1", event.NeedsInput, now))
	apply(t, m, fixture("wait2", event.NeedsInput, now.Add(time.Minute)))
	if m.Snapshot().NeedsYou[0].ID != "claude:wait1" {
		t.Fatal("wait ordering")
	}
	apply(t, m, fixture("alive", event.Done, now.Add(-time.Hour)))
	if m.Sessions["claude:alive"].Status != SessionWorking {
		t.Fatal("late event reversed state")
	}
}
func TestTemporaryIdentityNeverUsesCWD(t *testing.T) {
	e := fixture("", event.Working, time.Now())
	if _, err := SessionKey(e); err == nil {
		t.Fatal("cwd-only identity allowed")
	}
	e.ProcessID = 10
	e.ProcessIdentity = "start1"
	a, _ := SessionKey(e)
	e.CWD = "/other"
	b, _ := SessionKey(e)
	if a != b {
		t.Fatal("cwd affects identity")
	}
	e.ProcessIdentity = "start2"
	b, _ = SessionKey(e)
	if a == b {
		t.Fatal("PID reuse merged")
	}
}

func TestMultipleAgentsCountDistinctAttentionSessions(t *testing.T) {
	m := New(7)
	now := time.Now()
	apply(t, m, fixture("a", event.NeedsInput, now))
	apply(t, m, fixture("b", event.NeedsInput, now))
	e := fixture("c", event.Working, now)
	e.Source = "codex"
	apply(t, m, e)
	if v := m.Snapshot(); len(v.NeedsYou) != 2 || len(v.Working) != 1 {
		t.Fatal("expected badge 2 across three sessions", v)
	}
}

func TestAttentionLifecycleThroughResumeCompletionAndClosedSession(t *testing.T) {
	now := time.Now()
	m := New(7)
	apply(t, m, fixture("input", event.NeedsInput, now))
	apply(t, m, fixture("input", event.ToolActivity, now.Add(time.Second)))
	if len(m.Snapshot().NeedsYou) != 1 {
		t.Fatal("incidental activity cleared unresolved input")
	}
	apply(t, m, fixture("input", event.Working, now.Add(2*time.Second)))
	if len(m.Snapshot().NeedsYou) != 0 || len(m.Snapshot().Working) != 1 {
		t.Fatal("resume did not clear attention")
	}
	apply(t, m, fixture("input", event.Done, now.Add(3*time.Second)))
	if len(m.Snapshot().Working) != 0 || len(m.Snapshot().Recent) != 1 {
		t.Fatal("completion did not enter recent")
	}
	for _, kind := range []event.Type{event.NeedsInput, event.Error} {
		e := fixture(string(kind), kind, now)
		e.ProcessID = 123
		e.ProcessIdentity = "generation"
		apply(t, m, e)
	}
	m.Reconcile(now.Add(time.Hour), func(Session) bool { return true })
	if len(m.Snapshot().NeedsYou) != 2 {
		t.Fatal("time or viewing must not clear live attention")
	}
	m.Reconcile(now.Add(time.Hour), func(Session) bool { return false })
	if len(m.Snapshot().NeedsYou) != 0 || len(m.Snapshot().Recent) != 1 {
		t.Fatal("closed waiting/error session left a badge or fabricated completion")
	}
	apply(t, m, fixture("unidentified", event.NeedsInput, now))
	m.Reconcile(now.Add(23*time.Hour), func(Session) bool { return true })
	if len(m.Snapshot().NeedsYou) != 1 {
		t.Fatal("unidentified session expired prematurely")
	}
	m.Reconcile(now.Add(25*time.Hour), func(Session) bool { return true })
	if len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("unidentified attention did not expire")
	}
}

func TestRemoveOneRecentPreservesOtherSessions(t *testing.T) {
	m := New(7)
	now := time.Now()
	apply(t, m, fixture("a", event.Done, now))
	apply(t, m, fixture("b", event.Done, now))
	apply(t, m, fixture("active", event.NeedsInput, now))
	if err := m.RemoveRecent("claude:active"); err == nil {
		t.Fatal("removed unresolved session")
	}
	if err := m.RemoveRecent("claude:a"); err != nil {
		t.Fatal(err)
	}
	if len(m.Snapshot().Recent) != 1 || m.Snapshot().Recent[0].ID != "claude:b" || len(m.Snapshot().NeedsYou) != 1 {
		t.Fatal("single removal affected unrelated sessions")
	}
	if err := m.RemoveRecent("claude:a"); err != nil {
		t.Fatal("repeat removal must be harmless", err)
	}
}

func TestStopThenIdleStaysRecentWithoutNewAttentionOrCompletion(t *testing.T) {
	m := New(7)
	now := time.Now()
	stop := fixture("idle", event.Done, now)
	stop.Message = "Finished the requested work."
	apply(t, m, stop)
	idle := fixture("idle", event.Idle, now.Add(time.Minute))
	idle.Message = "Claude is waiting for your input"
	if changed, err := m.Apply(idle); err != nil || changed {
		t.Fatal("idle changed state", changed, err)
	}
	state := m.Snapshot()
	if len(state.NeedsYou) != 0 || len(state.Recent) != 1 || state.Recent[0].Summary != stop.Message || !state.Recent[0].FinishedAt.Equal(now) {
		t.Fatal("idle corrupted completion", state)
	}
	m.ClearRecent()
	apply(t, m, idle)
	if len(m.Snapshot().Recent) != 0 || len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("idle recreated cleared history")
	}
}
