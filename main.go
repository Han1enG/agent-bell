package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/adapter"
	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/install"
	"github.com/han1eng/agent-bell/internal/notify"
)

var version = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "AgentBell:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "test":
		return sendTest(stdout)
	case "notify":
		return notifyCommand(args[1:], stdin, stdout)
	case "doctor":
		return doctor(stdout)
	case "install":
		return installHooks(stdout)
	case "uninstall":
		return uninstallHooks(stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q (try agentbell help)", args[0])
	}
}

func installHooks(stdout io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate AgentBell executable: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate home directory: %w", err)
	}
	if err := install.New(homeDir, executable).Install(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "AgentBell\n\n✓ Claude Code hooks installed\n✓ Codex hooks installed")
	return nil
}

func uninstallHooks(stdout io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate home directory: %w", err)
	}
	if err := install.New(homeDir, "agentbell").Uninstall(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "AgentBell\n\n✓ AgentBell hooks removed\n\nYour existing settings were preserved.")
	return nil
}

func sendTest(stdout io.Writer) error {
	e := event.AgentEvent{Source: "agentbell", Type: event.Done, Project: "AgentBell", Title: "AgentBell", Message: "Notifications are working."}
	// The test command intentionally uses the same native notification path.
	if err := (notify.MacOS{}).Send(e); err != nil {
		return err
	}
	fmt.Fprint(stdout, "AgentBell\n\n🔔 Test notification\n\nNotifications are working.\n")
	return nil
}

func notifyCommand(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("notify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "hook source: claude or codex")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" {
		return errors.New("notify requires --source claude|codex")
	}
	payload, err := io.ReadAll(stdin)
	if err != nil {
		writeDebugLog("source=%s read_error=%v", *source, err)
		return fmt.Errorf("read hook payload: %w", err)
	}
	writeDebugLog("source=%s payload_bytes=%d", *source, len(payload))
	e, err := adapter.Parse(*source, payload)
	if err != nil {
		writeDebugLog("source=%s parse_error=%v", *source, err)
		return err
	}
	writeDebugLog("source=%s event=%s project=%s", e.Source, e.Type, e.Project)
	if err := (notify.MacOS{}).Send(e); err != nil {
		writeDebugLog("source=%s event=%s notify_error=%v", e.Source, e.Type, err)
		return err
	}
	writeDebugLog("source=%s event=%s notify_ok=true", e.Source, e.Type)
	fmt.Fprintf(stdout, "notified %s %s for %s\n", e.Source, e.Type, e.Project)
	return nil
}

func writeDebugLog(format string, args ...any) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(homeDir, "Library", "Logs", "AgentBell", "agentbell.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func doctor(stdout io.Writer) error {
	homeDir, _ := os.UserHomeDir()
	executable, _ := os.Executable()
	claudeHooks, claudeMissing, claudeErr := install.HasAgentBellHooks(filepath.Join(homeDir, ".claude", "settings.json"), []string{"Notification", "PermissionRequest", "Stop", "StopFailure"})
	codexHooks, codexMissing, codexErr := install.HasAgentBellHooks(filepath.Join(homeDir, ".codex", "hooks.json"), []string{"PermissionRequest", "Stop"})
	checks := []struct {
		name string
		ok   bool
		info string
	}{
		{"macOS", runtime.GOOS == "darwin", runtime.GOOS},
		{"Native helper", notify.NativeHelperFor(executable) != "", notify.NativeHelperFor(executable)},
		{"Claude Code", commandAvailable("claude"), commandVersion("claude")},
		{"Codex", commandAvailable("codex"), commandVersion("codex")},
		{"Claude hooks", claudeErr == nil && claudeHooks, doctorHookInfo(claudeMissing, claudeErr)},
		{"Codex hooks", codexErr == nil && codexHooks, doctorHookInfo(codexMissing, codexErr)},
	}
	fmt.Fprint(stdout, "AgentBell Doctor\n\n")
	ok := true
	for _, check := range checks {
		mark := "✓"
		if !check.ok {
			mark = "✗"
			ok = false
		}
		fmt.Fprintf(stdout, "%s %-12s %s\n", mark, check.name, check.info)
	}
	if ok {
		fmt.Fprintln(stdout, "\nStatus\n✓ Everything looks good")
		return nil
	}
	fmt.Fprintln(stdout, "\nStatus\n✗ Some checks need attention")
	return nil
}

func doctorHookInfo(missing string, err error) string {
	if err != nil {
		return err.Error()
	}
	if missing != "" {
		return "missing " + missing
	}
	return "configured"
}

func commandAvailable(name string) bool { _, err := exec.LookPath(name); return err == nil }

func commandVersion(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return "not installed"
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return "installed (version unavailable)"
	}
	return strings.TrimSpace(string(out))
}

func printUsage(w io.Writer) {
	usage := `AgentBell - lightweight coding agent notifications

Usage:
  agentbell test
  agentbell notify --source claude|codex < event.json
  agentbell doctor
  agentbell version
  agentbell install
  agentbell uninstall
`
	fmt.Fprint(w, usage)
}
