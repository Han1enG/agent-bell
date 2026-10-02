package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDoctorExitCodes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty-path"))
	t.Setenv("AGENTBELL_NOTIFIER", "")
	var output bytes.Buffer
	err := doctor(nil, &output)
	want := 1
	if runtime.GOOS != "darwin" {
		want = 2
	}
	var exit doctorExit
	if !errors.As(err, &exit) || exit.code != want {
		t.Fatalf("doctor error=%v, want exit %d", err, want)
	}
	if output.Len() == 0 {
		t.Fatal("doctor produced no diagnostics")
	}
}

func TestInstallDryRunIsReadOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "agentbell")
	var output bytes.Buffer
	if err := run([]string{"install", "--dry-run"}, strings.NewReader(""), &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No files were changed") {
		t.Fatalf("unexpected preview: %s", output.String())
	}
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Fatal("dry-run created the AgentBell config directory")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run created Claude settings")
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run created Codex hooks")
	}
}

func TestUnknownHookEventIsIgnoredWithoutNotification(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var output bytes.Buffer
	err := notifyCommand([]string{"--source", "codex"}, strings.NewReader(`{"hook_event_name":"FutureEvent"}`), &output)
	if err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("unknown event produced output: %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "Caches", "AgentBell")); !os.IsNotExist(err) {
		t.Fatal("unknown event wrote debounce cache")
	}
}

func TestCodexPermissionRequestIsSilentByDefault(t *testing.T) {
	for _, contents := range []string{"", "[notifications]\nneeds_approval = true\n", "[notifications]\nneeds_approval = invalid\n"} {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Join(t.TempDir(), "empty-path"))
		t.Setenv("AGENTBELL_NOTIFIER", "")
		if contents != "" {
			path := filepath.Join(home, ".config", "agentbell", "config.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var output bytes.Buffer
		if err := notifyCommand([]string{"--source", "codex"}, strings.NewReader(`{"hook_event_name":"PermissionRequest","session_id":"auto-reviewed","cwd":"/tmp"}`), &output); err != nil {
			t.Fatal(err)
		}
		if output.Len() != 0 {
			t.Fatalf("suppressed approval wrote output: %s", &output)
		}
		if _, err := os.Stat(filepath.Join(home, "Library", "Caches", "AgentBell")); !os.IsNotExist(err) {
			t.Fatal("suppressed request should not reach notification/debounce")
		}
	}
}
