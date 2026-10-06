package attention

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/surface"
)

func TestSessionEndClearsEveryStatusButStopKeepsRuntime(t *testing.T) {
	for _, typ := range []event.Type{event.NeedsInput, event.NeedsApproval, event.Working, event.Done, event.Error} {
		t.Run(string(typ), func(t *testing.T) {
			now := time.Now()
			m := New(7)
			e := fixture("a", event.SessionStarted, now)
			e.ProcessID = 123
			e.ProcessIdentity = "A"
			e.AgentFlavor = "claude_cli"
			apply(t, m, e)
			e.Type = typ
			e.AttentionConfirmed = true
			e.Timestamp = now.Add(time.Second)
			apply(t, m, e)
			if typ == event.Done && m.Sessions["claude:a"].RuntimeState != RuntimeRunning {
				t.Fatal("Stop ended runtime")
			}
			e.Type = event.SessionEnded
			e.ExitReason = "prompt_input_exit"
			e.Timestamp = now.Add(2 * time.Second)
			apply(t, m, e)
			s := m.Sessions["claude:a"]
			v := m.Snapshot()
			if s.RuntimeState != RuntimeExited || s.Status != SessionClosed || len(v.NeedsYou) != 0 || len(v.Working) != 0 || len(v.Recent) != 0 || len(v.Closed) != 1 || s.LastExitReason != e.ExitReason {
				t.Fatal(s, v)
			}
		})
	}
}
func TestDismissCannotBeRevivedByLateAttentionOrDuplicateStart(t *testing.T) {
	for _, typ := range []event.Type{event.NeedsInput, event.Working, event.Done, event.Error, event.SessionEnded} {
		now := time.Now()
		m := New(7)
		e := fixture("a", event.SessionStarted, now)
		e.ProcessID = 123
		e.ProcessIdentity = "A"
		apply(t, m, e)
		e.Type = typ
		e.Timestamp = now.Add(time.Second)
		apply(t, m, e)
		m.Dismiss("claude:a", now.Add(2*time.Second))
		for _, late := range []event.Type{event.NeedsInput, event.ToolActivity, event.Done, event.Error, event.Working} {
			e.Type = late
			e.Timestamp = now.Add(3 * time.Second)
			apply(t, m, e)
		}
		if s := m.Snapshot(); len(s.NeedsYou)+len(s.Working)+len(s.Recent)+len(s.Closed) != 0 {
			t.Fatal("late event revived dismissal", s)
		}
		// A distinct runtime can show the same native conversation again.
		e.Type = event.SessionStarted
		e.ProcessID = 124
		e.ProcessIdentity = "B"
		e.Timestamp = now.Add(4 * time.Second)
		apply(t, m, e)
		if len(m.Snapshot().Working) != 1 {
			t.Fatal("fresh instance hidden")
		}
	}
}
func TestDuplicateSessionStartDoesNotClearRealAttention(t *testing.T) {
	now := time.Now()
	m := New(7)
	e := fixture("a", event.SessionStarted, now)
	e.ProcessID = 123
	e.ProcessIdentity = "A"
	apply(t, m, e)
	e.Type = event.NeedsInput
	e.Timestamp = now.Add(time.Second)
	apply(t, m, e)
	m.Dismiss("claude:a", now.Add(2*time.Second))
	e.Type = event.SessionStarted
	e.Timestamp = now.Add(3 * time.Second)
	apply(t, m, e)
	if len(m.Snapshot().Working) != 0 || m.Sessions["claude:a"].DismissedAt == nil {
		t.Fatal("duplicate start revived dismissal")
	}
}
func TestResumeReplacesIdentityAndRejectsOldRuntimeEvents(t *testing.T) {
	now := time.Now()
	m := New(7)
	e := fixture("a", event.SessionStarted, now)
	e.ProcessID = 123
	e.ProcessIdentity = "A"
	e.SessionTitle = "Keep title"
	e.ReturnTarget = &surface.ReturnTarget{ContextID: "old"}
	apply(t, m, e)
	e.Type = event.SessionEnded
	e.Timestamp = now.Add(time.Second)
	apply(t, m, e)
	e.Type = event.SessionStarted
	e.ProcessID = 124
	e.ProcessIdentity = "B"
	e.Timestamp = now.Add(2 * time.Second)
	e.SessionTitle = ""
	e.ReturnTarget = &surface.ReturnTarget{ContextID: "new"}
	apply(t, m, e)
	e.Type = event.NeedsInput
	e.ProcessID = 123
	e.ProcessIdentity = "A"
	e.Timestamp = now.Add(3 * time.Second)
	apply(t, m, e)
	s := m.Sessions["claude:a"]
	if len(m.Sessions) != 1 || s.ProcessIdentity != "B" || s.ReturnTarget.ContextID != "new" || s.Attention != AttentionNone || s.Title != "Keep title" || s.PreviousRuntimeInstanceID == "" {
		t.Fatal(s)
	}
}
func TestConcurrentRuntimesRemainDistinct(t *testing.T) {
	now := time.Now()
	m := New(7)
	e := fixture("a", event.SessionStarted, now)
	e.ProcessID = 123
	e.ProcessIdentity = "A"
	apply(t, m, e)
	e.ProcessID = 124
	e.ProcessIdentity = "B"
	e.Timestamp = now.Add(time.Second)
	apply(t, m, e)
	if len(m.Snapshot().Working) != 2 {
		t.Fatal("parallel runtimes merged")
	}
	e.Type = event.SessionEnded
	e.Timestamp = now.Add(2 * time.Second)
	apply(t, m, e)
	if len(m.Snapshot().Working) != 1 || len(m.Snapshot().Closed) != 1 {
		t.Fatal("wrong instance ended")
	}
}
func TestUnknownNeverExpiresAndSurfaceLossDoesNotCloseRuntime(t *testing.T) {
	m := New(7)
	now := time.Now()
	apply(t, m, fixture("a", event.NeedsInput, now))
	m.Reconcile(now.Add(48*time.Hour), func(Session) ProbeResult { return Unknown })
	if len(m.Snapshot().NeedsYou) != 1 || m.Sessions["claude:a"].RuntimeState != RuntimeUnknown {
		t.Fatal("unknown falsely exited")
	}
	m.Reconcile(now.Add(49*time.Hour), func(Session) ProbeResult { return Alive })
	s := m.Sessions["claude:a"]
	s.SurfaceState = "unavailable"
	m.Sessions[s.ID] = s
	if len(m.Snapshot().NeedsYou) != 1 || s.RuntimeState != RuntimeRunning {
		t.Fatal("surface loss killed attention")
	}
	m.Reconcile(now.Add(50*time.Hour), func(Session) ProbeResult { return Exited })
	if len(m.Snapshot().NeedsYou) != 0 || len(m.Snapshot().Closed) != 1 {
		t.Fatal("real exit left attention")
	}
}
func TestProbeFailureAndPIDReuse(t *testing.T) {
	if classifyProcessProbe("A", []byte(" B\n"), nil) != Exited {
		t.Fatal("PID reuse")
	}
	if classifyProcessProbe("A", []byte("A\n"), nil) != Alive {
		t.Fatal("alive")
	}
	if classifyProcessProbe("A", nil, errors.New("permission denied")) != Unknown {
		t.Fatal("permission inferred exit")
	}
	ctxErr := errors.New("timeout")
	if classifyProcessProbe("A", nil, ctxErr) != Unknown {
		t.Fatal("timeout inferred exit")
	}
	// Exercise actual absent-PID exit semantics, without touching a user process.
	child := exec.Command("/usr/bin/true")
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/ps", "-p", fmt.Sprint(child.Process.Pid), "-o", "lstart=")
	b, err := cmd.Output()
	if classifyProcessProbe("A", b, err) != Exited {
		t.Fatal("absent PID not exited", err)
	}
	if ProcessProbe(Session{}) != Unknown {
		t.Fatal("missing identity not unknown")
	}
}
func TestRecoveryRequiresNativeUUIDFlavorExitedRuntimeAndSafeCLI(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(cwd, "cli")
	os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700)
	s := Session{RuntimeState: RuntimeExited, NativeSessionID: "12345678-1234-1234-1234-123456789abc", AgentFlavor: "claude_cli", CWD: cwd}
	target, err := ResumeArguments(s, path)
	if err != nil || len(target.Argv) != 2 || target.Argv[0] != "--resume" {
		t.Fatal(target, err)
	}
	s.AgentFlavor = "codex_cli"
	target, err = ResumeArguments(s, path)
	if err != nil || target.Argv[0] != "resume" {
		t.Fatal(target, err)
	}
	for _, change := range []func(*Session){func(s *Session) { s.RuntimeState = RuntimeRunning }, func(s *Session) { s.NativeSessionID = "temporary:hash" }, func(s *Session) { s.AgentFlavor = "codex_desktop" }, func(s *Session) { s.CWD = filepath.Join(cwd, "deleted") }} {
		bad := s
		change(&bad)
		if _, err := ResumeArguments(bad, path); err == nil {
			t.Fatal("unsafe recovery allowed", bad)
		}
	}
	if _, err := ResumeArguments(s, filepath.Join(cwd, "missing")); err == nil {
		t.Fatal("missing CLI allowed")
	}
	if ShellQuote("x'$(touch /tmp/pwn)") != "'x'\"'\"'$(touch /tmp/pwn)'" {
		t.Fatal("unsafe quoting")
	}
}
func TestClosedLimitAndSevenDayRetention(t *testing.T) {
	m := New(7)
	now := time.Now()
	for i := 0; i < 8; i++ {
		e := fixture(string(rune('a'+i)), event.SessionEnded, now.Add(time.Duration(i)*time.Second))
		apply(t, m, e)
	}
	if len(m.Snapshot().Closed) != 5 || len(m.Snapshot().Recent) != 0 {
		t.Fatal("closed leaked into READY")
	}
	m.Cleanup(now.AddDate(0, 0, 8))
	if len(m.Sessions) != 0 {
		t.Fatal("closed retention failed")
	}
}

func TestClosedOrderingAndLimitAreStableWhenExitTimesMatch(t *testing.T) {
	m := New(7)
	now := time.Now()
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("claude:%02d", i)
		m.Sessions[id] = Session{ID: id, Status: SessionClosed, RuntimeState: RuntimeExited, Attention: AttentionNone, UpdatedAt: now}
	}
	m.Sessions["newest"] = Session{ID: "newest", Status: SessionClosed, RuntimeState: RuntimeExited, Attention: AttentionNone, UpdatedAt: now.Add(time.Second)}
	m.Sessions["oldest"] = Session{ID: "oldest", Status: SessionClosed, RuntimeState: RuntimeExited, Attention: AttentionNone, UpdatedAt: now.Add(-time.Second)}
	for refresh := 0; refresh < 200; refresh++ {
		closed := m.Snapshot().Closed
		var ids []string
		for _, session := range closed {
			ids = append(ids, session.ID)
		}
		if got := strings.Join(ids, ","); got != "newest,claude:00,claude:01,claude:02,claude:03" {
			t.Fatalf("refresh %d changed order or limited membership: %s", refresh, got)
		}
	}
}

func TestExpiredDismissalKeepsOnlyWatermarkAndCannotRevive(t *testing.T) {
	now := time.Now()
	m := New(7)
	e := fixture("a", event.NeedsInput, now)
	e.Message = "summary"
	e.ReturnTarget = &surface.ReturnTarget{ContextID: "old"}
	apply(t, m, e)
	m.Dismiss("claude:a", now)
	m.Cleanup(now.AddDate(0, 0, 8))
	s := m.Sessions["claude:a"]
	if s.DismissedAt == nil || s.Summary != "" || s.ReturnTarget != nil || s.CWD != "" {
		t.Fatal("heavy history not compacted", s)
	}
	e.Timestamp = now.AddDate(0, 0, 9)
	apply(t, m, e)
	if len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("compacted tombstone resurrected")
	}
}

func TestRecoveryChecksInstalledCommandContractAndKeepsFailedRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bin, "claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' '--resume SESSION_ID'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(home, "project with ' quote")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	s := Session{ID: "internal", RuntimeState: RuntimeExited, AgentFlavor: "claude_cli", NativeSessionID: "12345678-1234-1234-1234-123456789abc", CWD: cwd, Title: "Same title", Summary: "Keep history"}
	result := Recovery(s)
	if result.Command == "" || result.Target == nil || !strings.Contains(result.Command, ShellQuote(cwd)) {
		t.Fatal(result)
	}
	other := s
	other.NativeSessionID = "12345678-1234-1234-1234-123456789abd"
	if Recovery(other).Command == result.Command {
		t.Fatal("same project/title picked wrong native ID")
	}
	s.ConcurrentRuntime = true
	if Recovery(s).Command != "" {
		t.Fatal("parallel live runtime allowed")
	}
	s.ConcurrentRuntime = false
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' 'incompatible CLI'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	failed := Recovery(s)
	if failed.Command != "" || failed.Reason == "" || s.Title != "Same title" || s.Summary != "Keep history" {
		t.Fatal("incompatible CLI did not preserve record", failed, s)
	}
}

func TestClosedMenuDoesNotOfferCopyWhenCWDIsMissingOrAnotherRuntimeExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(bin, 0700)
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0700)
	s := Session{ID: "closed", Agent: "claude", RuntimeState: RuntimeExited, RecoveryCapability: "supported", NativeSessionID: "12345678-1234-1234-1234-123456789abc", AgentFlavor: "claude_cli", CWD: home}
	if action, _ := SessionAction(s); action != "copy_resume_command" && action != "resume_in_tabby" {
		t.Fatal("valid local command hidden", action)
	}
	s.CWD = filepath.Join(home, "missing")
	if action, _ := SessionAction(s); action == "copy_resume_command" || action == "resume_in_tabby" {
		t.Fatal("missing CWD offered copy")
	}
	s.CWD = home
	s.Status = SessionClosed
	m := New(7)
	m.Sessions[s.ID] = s
	live := s
	live.ID = "running"
	live.RuntimeState = RuntimeRunning
	live.Status = SessionWorking
	m.Sessions[live.ID] = live
	if action := m.Snapshot().Closed[0].Action; action == "copy_resume_command" || action == "resume_in_tabby" {
		t.Fatal("known parallel runtime ignored")
	}
	s.AgentFlavor = "codex_desktop"
	s.ReturnTarget = &surface.ReturnTarget{AppBundleID: "com.openai.codex", Capability: surface.ReturnApp}
	if action, _ := SessionAction(s); action != "open_app" {
		t.Fatal("Desktop action must only open App")
	}
}

func TestTrackingCapacityNeverEvictsWatermarksAndDismissStillWorks(t *testing.T) {
	now := time.Now()
	m := New(7)
	for i := 0; i < MaxTrackedSessions; i++ {
		id := fmt.Sprintf("claude:%d", i)
		m.Sessions[id] = Session{ID: id, Agent: "claude", Status: SessionNeedsYou, Attention: AttentionInput, UpdatedAt: now}
	}
	e := fixture("over-capacity", event.NeedsInput, now)
	if _, err := m.Apply(e); !errors.Is(err, ErrTrackingCapacity) {
		t.Fatal("new identities exceeded capacity", err)
	}
	if len(m.Sessions) != MaxTrackedSessions || m.Snapshot().TrackingWarning == "" {
		t.Fatal("capacity boundary missing")
	}
	m.ClearAll(now)
	if len(m.Snapshot().NeedsYou) != 0 || len(m.Sessions) != MaxTrackedSessions {
		t.Fatal("capacity prevented Dismiss or evicted watermarks")
	}
}

func TestWorstCaseCompactedTombstoneFitsByteBudget(t *testing.T) {
	now := time.Now()
	m := New(7)
	id := "claude:" + strings.Repeat("\x00", 512)
	m.Sessions[id] = Session{ID: id, Agent: "claude", NativeSessionID: strings.Repeat("\x00", 512), ProcessIdentity: strings.Repeat("\x00", 256), RuntimeInstanceID: strings.Repeat("\x00", 300), DismissedAt: &now, UpdatedAt: now, Title: "private", CWD: "/private", Summary: "private", ExitReason: "private", LastExitReason: "private"}
	m.Cleanup(now.AddDate(0, 0, 8))
	b, err := json.Marshal(m.Sessions[id])
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > MaxTombstoneJSONBytes || strings.Contains(string(b), "private") {
		t.Fatal("tombstone byte/privacy bound failed", len(b))
	}
}
