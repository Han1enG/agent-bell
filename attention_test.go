//go:build cgo

package main

import (
	"bytes"
	"encoding/json"
	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/config"
	"github.com/han1eng/agent-bell/internal/event"
	"io"
	"os"
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
	payload := `{"hook_event_name":"Notification","notification_type":"idle_prompt","session_id":"hook","cwd":"/tmp"}`
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
