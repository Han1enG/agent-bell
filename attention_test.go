//go:build cgo

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/config"
	"github.com/han1eng/agent-bell/internal/event"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeOutput struct {
	sync.Mutex
	bytes.Buffer
}

func (b *safeOutput) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}
func hostHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "abh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	path := config.Path(home)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("[notifications]\ndone=false\nneeds_input=false\nneeds_approval=false\nerror=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return home
}
func startHost(t *testing.T, home string) (io.WriteCloser, chan error) {
	t.Helper()
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	out := &safeOutput{}
	go func() { done <- attentionHost(reader, out) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "status"}, 100*time.Millisecond); err == nil {
			return writer, done
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("host startup timeout")
	return nil, nil
}
func sendEvent(t *testing.T, home, id string, typ event.Type) {
	t.Helper()
	e := event.AgentEvent{Source: "claude", SessionID: id, Project: id, Type: typ, Timestamp: time.Now()}
	if _, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Event: &e}, time.Second); err != nil {
		t.Fatal(err)
	}
}
func closeHost(t *testing.T, w io.WriteCloser, done chan error) {
	t.Helper()
	w.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("host failed to exit on app EOF")
	}
}
func TestHostTrackingPauseRestartAndJSON(t *testing.T) {
	home := hostHome(t)
	w, done := startHost(t, home)
	sendEvent(t, home, "a", event.Working)
	sendEvent(t, home, "b", event.NeedsInput)
	sendEvent(t, home, "c", event.Done)
	sendEvent(t, home, "d", event.Working)
	if err := attentionControl([]string{"pause"}); err != nil {
		t.Fatal(err)
	}
	sendEvent(t, home, "e", event.Working)
	var out bytes.Buffer
	if err := statusCommand([]string{"--json"}, &out); err != nil {
		t.Fatal(err)
	}
	var snapshot attention.Snapshot
	if err := json.Unmarshal(out.Bytes(), &snapshot); err != nil {
		t.Fatal("stdout not pure JSON", err)
	}
	if len(snapshot.NeedsYou) != 1 || len(snapshot.Working) != 3 || len(snapshot.Recent) != 1 || !snapshot.Paused {
		t.Fatal("paused tracking or grouping failed", out.String())
	}
	closeHost(t, w, done)
	snapshot, err := attentionStatus(home)
	if err != nil || len(snapshot.NeedsYou) != 1 || len(snapshot.Working) != 3 {
		t.Fatal("offline SQLite restore", snapshot, err)
	}
	w, done = startHost(t, home)
	defer closeHost(t, w, done)
	snapshot, err = attentionStatus(home)
	if err != nil || !snapshot.Paused || len(snapshot.NeedsYou) != 1 {
		t.Fatal("app restart lost state", snapshot, err)
	}
	sendEvent(t, home, "b", event.Working)
	snapshot, _ = attentionStatus(home)
	if len(snapshot.NeedsYou) != 0 {
		t.Fatal("resume did not clear badge")
	}
	if err = attentionControl([]string{"clear_recent"}); err != nil {
		t.Fatal(err)
	}
	snapshot, _ = attentionStatus(home)
	if len(snapshot.Recent) != 0 || len(snapshot.Working) != 4 {
		t.Fatal("clear destroyed working")
	}
}
func TestHookIPCConfigAndMissingHostFallback(t *testing.T) {
	home := hostHome(t)
	w, done := startHost(t, home)
	var out bytes.Buffer
	payload := `{"hook_event_name":"Notification","notification_type":"agent_needs_input","session_id":"hook","cwd":"/tmp"}`
	if err := notifyCommand([]string{"--source", "claude"}, strings.NewReader(payload), &out); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := attentionStatus(home)
	if len(snapshot.NeedsYou) != 1 {
		t.Fatal("notification config suppressed state")
	}
	closeHost(t, w, done)
	// Capture the existing fallback helper invocation without posting a real alert.
	helper := filepath.Join(home, "AgentBellNotifier")
	capture := filepath.Join(home, "fallback.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + capture + "'\n"
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	if err := os.WriteFile(config.Path(home), []byte("[notifications]\ndone=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := notifyCommand([]string{"--source", "codex"}, strings.NewReader(`{"hook_event_name":"Stop","session_id":"fallback","cwd":"/tmp"}`), &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture)
	if err != nil || !strings.Contains(string(data), "Task completed") {
		t.Fatal("legacy fallback missing", string(data), err)
	}
}
func TestStoreFailureKeepsInMemoryTracking(t *testing.T) {
	home := hostHome(t)
	if err := os.MkdirAll(attention.DBPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	w, done := startHost(t, home)
	defer closeHost(t, w, done)
	sendEvent(t, home, "live", event.NeedsInput)
	snapshot, err := attentionStatus(home)
	if err != nil || len(snapshot.NeedsYou) != 1 || snapshot.StorageError == "" {
		t.Fatal("storage failure stopped state", snapshot, err)
	}
}

func TestResidentNotificationStreamKeepsTrackingAndPauseIndependent(t *testing.T) {
	home := hostHome(t)
	if err := os.WriteFile(config.Path(home), []byte("[attention_center]\ndone_notifications=true\n[notifications]\ndone=true\nneeds_input=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	output := &safeOutput{}
	done := make(chan error, 1)
	go func() { done <- attentionHost(reader, output) }()
	defer closeHost(t, writer, done)
	deadline := time.Now().Add(3 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if _, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "status"}, 100*time.Millisecond); err == nil {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("host unavailable")
	}
	sendEvent(t, home, "completed", event.Done)
	notificationCount := func() int {
		output.Lock()
		text := output.Buffer.String()
		output.Unlock()
		count := 0
		for _, line := range strings.Split(text, "\n") {
			var value map[string]any
			if json.Unmarshal([]byte(line), &value) == nil && value["kind"] == "notification" {
				count++
				if value["title"] != "Claude · completed" || value["session_id"] != "completed" {
					t.Errorf("lost native notification metadata: %v", value)
				}
			}
		}
		return count
	}
	deadline = time.Now().Add(time.Second)
	for notificationCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if notificationCount() != 1 {
		t.Fatal("resident delivery missing or duplicated")
	}
	if err := attentionControl([]string{"pause"}); err != nil {
		t.Fatal(err)
	}
	sendEvent(t, home, "paused-input", event.NeedsInput)
	state, err := attentionStatus(home)
	if err != nil || len(state.NeedsYou) != 1 || len(state.Recent) != 1 {
		t.Fatalf("notification pause changed tracking: %+v %v", state, err)
	}
	time.Sleep(50 * time.Millisecond)
	if notificationCount() != 1 {
		t.Fatal("paused event still posted a notification")
	}
}

func TestResidentDoneDefaultIsQuietButFallbackRemainsEnabled(t *testing.T) {
	cfg := config.Defaults()
	if residentNotificationAllowed(cfg, event.AgentEvent{Type: event.Done}) {
		t.Fatal("resident Done default must be quiet")
	}
	if !cfg.Allows("done") {
		t.Fatal("quit/CLI-only Done fallback must stay enabled")
	}
	for _, kind := range []event.Type{event.NeedsInput, event.Error} {
		if !residentNotificationAllowed(cfg, event.AgentEvent{Type: kind}) {
			t.Fatal("attention alert suppressed", kind)
		}
	}
	cfg.AttentionCenter.DoneNotifications = true
	if !residentNotificationAllowed(cfg, event.AgentEvent{Type: event.Done}) {
		t.Fatal("explicit completion banners ignored")
	}
}

func TestSingleRecentRemovalThroughIPCAndRestore(t *testing.T) {
	home := hostHome(t)
	w, done := startHost(t, home)
	sendEvent(t, home, "first", event.Done)
	sendEvent(t, home, "second", event.Done)
	sendEvent(t, home, "waiting", event.NeedsInput)
	before, err := attentionStatus(home)
	if err != nil || len(before.ObservedAgents) != 1 || before.ObservedAgents[0] != "claude" {
		t.Fatal("real event not reported", before, err)
	}
	if err := attentionControl([]string{"remove_recent", "claude:waiting"}); err == nil {
		t.Fatal("deleted attention through IPC")
	}
	if err := attentionControl([]string{"remove_recent", "claude:first"}); err != nil {
		t.Fatal(err)
	}
	closeHost(t, w, done)
	restored, err := attentionStatus(home)
	if err != nil || len(restored.Recent) != 1 || restored.Recent[0].ID != "claude:second" || len(restored.NeedsYou) != 1 {
		t.Fatal("removal was not selective or persisted", restored, err)
	}
	w, done = startHost(t, home)
	defer closeHost(t, w, done)
	fresh, err := attentionStatus(home)
	if err != nil || len(fresh.ObservedAgents) != 0 {
		t.Fatal("restored history claimed fresh hook delivery", fresh, err)
	}
}

func TestDismissAndClearAllPersistBeforeAcknowledgement(t *testing.T) {
	home := hostHome(t)
	w, done := startHost(t, home)
	sendEvent(t, home, "wait", event.NeedsInput)
	sendEvent(t, home, "work", event.Working)
	sendEvent(t, home, "ready", event.Done)
	if err := attentionControl([]string{"dismiss_session", "claude:wait"}); err != nil {
		t.Fatal(err)
	}
	store, err := attention.OpenStore(attention.DBPath(home), true)
	if err != nil {
		t.Fatal(err)
	}
	m := attention.New(7)
	if err := store.Load(m); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if m.Sessions["claude:wait"].DismissedAt == nil || len(m.Snapshot().NeedsYou) != 0 {
		t.Fatal("ack preceded persistence")
	}
	sendEvent(t, home, "wait", event.NeedsInput)
	if state, _ := attentionStatus(home); len(state.NeedsYou) != 0 {
		t.Fatal("late hook revived badge")
	}
	if err := attentionControl([]string{"clear_all"}); err != nil {
		t.Fatal(err)
	}
	closeHost(t, w, done)
	w, done = startHost(t, home)
	defer closeHost(t, w, done)
	if state, _ := attentionStatus(home); len(state.Working)+len(state.Recent)+len(state.NeedsYou) != 0 {
		t.Fatal("restart revived hidden sessions")
	}
}

func TestRealProcessExitClearsAttentionWithinPollingCycle(t *testing.T) {
	home := hostHome(t)
	w, done := startHost(t, home)
	defer closeHost(t, w, done)
	child := exec.Command("/bin/sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	output, err := exec.Command("/bin/ps", "-p", fmt.Sprint(child.Process.Pid), "-o", "lstart=").Output()
	if err != nil {
		t.Fatal(err)
	}
	e := event.AgentEvent{Source: "claude", Type: event.NeedsInput, SessionID: "real-process", ProcessID: child.Process.Pid, ProcessIdentity: strings.Join(strings.Fields(string(output)), " "), Timestamp: time.Now()}
	if _, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Event: &e}, time.Second); err != nil {
		t.Fatal(err)
	}
	if state, _ := attentionStatus(home); len(state.NeedsYou) != 1 {
		t.Fatal("fixture not waiting")
	}
	child.Process.Kill()
	child.Wait()
	deadline := time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		state, _ := attentionStatus(home)
		if len(state.NeedsYou) == 0 && len(state.Closed) == 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("reliable process exit was not reconciled within one polling cycle")
}

func TestHooksRemainResponsiveWhileThousandsOfTombstonesFlush(t *testing.T) {
	home := hostHome(t)
	store, err := attention.OpenStore(attention.DBPath(home), false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().AddDate(0, 0, -8)
	m := attention.New(7)
	for i := 0; i < 6000; i++ {
		id := fmt.Sprintf("claude:%d", i)
		m.Sessions[id] = attention.Session{ID: id, Agent: "claude", Status: attention.SessionNeedsYou, Attention: attention.AttentionInput, RuntimeState: attention.RuntimeUnknown, UpdatedAt: now, DismissedAt: &now}
	}
	m.Cleanup(time.Now())
	if err := store.Save(m, nil); err != nil {
		t.Fatal(err)
	}
	store.Close()
	w, done := startHost(t, home)
	defer closeHost(t, w, done)
	sendEvent(t, home, "live", event.Working)
	start := time.Now()
	max := time.Duration(0)
	// Cross multiple 200ms flush cycles while the store serializes 6000 watermarks.
	for i := 0; i < 250; i++ {
		e := event.AgentEvent{Source: "claude", Type: event.ToolActivity, SessionID: "live", Timestamp: time.Now()}
		at := time.Now()
		if _, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Event: &e}, 200*time.Millisecond); err != nil {
			t.Fatal("hooks blocked by tombstones", err)
		}
		elapsed := time.Since(at)
		if elapsed > max {
			max = elapsed
		}
		if i%10 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Logf("250 hooks across flush cycles: total=%s max-ack=%s", time.Since(start), max)
	if state, err := attentionStatus(home); err != nil || len(state.Working) != 1 || len(state.NeedsYou) != 0 {
		t.Fatal(state, err)
	}
}
