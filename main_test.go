package main

import (
	"bytes"
	"errors"
	"github.com/han1eng/agent-bell/internal/install"
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

func TestInstallSkipIntegrationAndExplicitEnable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("installer is macOS-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	helper := filepath.Join(home, "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	var output bytes.Buffer
	if err := run([]string{"install", "--skip-tabby"}, strings.NewReader(""), &output, &output); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(home, "Library", "Application Support", "tabby", "plugins", "node_modules", "tabby-agentbell", "index.js")
	if _, err := os.Stat(plugin); !os.IsNotExist(err) {
		t.Fatal("skip flag installed plugin")
	}
	if err := run([]string{"install", "--tabby"}, strings.NewReader(""), &output, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plugin); err != nil {
		t.Fatal("explicit flag did not install bundled plugin", err)
	}
	if err := run([]string{"install", "--tabby", "--skip-tabby"}, strings.NewReader(""), &output, &output); err == nil {
		t.Fatal("conflicting flags accepted")
	}
}

func TestDoctorHealthyInstallDoesNotRequireCurrentContext(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS doctor")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	commands := filepath.Join(home, "commands")
	if err := os.Mkdir(commands, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex", "AgentBellNotifier"} {
		body := "#!/bin/sh\nprintf 'fixture-version\\n'\n"
		if name == "AgentBellNotifier" {
			body = "#!/bin/sh\nprintf 'authorized\\n'\n"
		}
		if err := os.WriteFile(filepath.Join(commands, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", commands)
	t.Setenv("AGENTBELL_NOTIFIER", filepath.Join(commands, "AgentBellNotifier"))
	t.Setenv("AGENTBELL_SURFACE", "future")
	t.Setenv("AGENTBELL_CONTEXT_ID", "")
	t.Setenv("TERM_PROGRAM", "")
	i := install.New(home, "agentbell")
	if err := i.InstallClaude(); err != nil {
		t.Fatal(err)
	}
	if err := i.InstallCodex(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doctor(nil, &out); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "Exact session return unavailable") || !strings.Contains(out.String(), "Everything looks good") {
		t.Fatal(out.String())
	}
}
