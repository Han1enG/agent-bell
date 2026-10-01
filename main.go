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
	"github.com/han1eng/agent-bell/internal/config"
	"github.com/han1eng/agent-bell/internal/debounce"
	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/install"
	"github.com/han1eng/agent-bell/internal/notify"
)

var version = "0.2.0"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "AgentBell:", err)
		var exit doctorExit
		if errors.As(err, &exit) {
			os.Exit(exit.code)
		}
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
		return doctor(args[1:], stdout)
	case "install":
		if len(args) == 2 && args[1] == "--dry-run" {
			return installPreview(stdout)
		}
		if len(args) > 1 {
			return fmt.Errorf("unknown install option %q", args[1])
		}
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
		if errors.Is(err, adapter.ErrUnknownEvent) {
			writeDebugLog("source=%s unknown_event=%v ignored=true", *source, err)
			return nil
		}
		writeDebugLog("source=%s parse_error=%v", *source, err)
		return err
	}
	writeDebugLog("source=%s event=%s project=%s", e.Source, e.Type, e.Project)
	homeDir, _ := os.UserHomeDir()
	configPath := config.Path(homeDir)
	cfg, configErr := config.Load(configPath)
	if configErr != nil {
		writeDebugLog("config_path=%s config_error=%v using_defaults=true", configPath, configErr)
		cfg = config.Defaults()
	} else {
		writeDebugLog("config_path=%s loaded=true", configPath)
	}
	if !cfg.Allows(string(e.Type)) {
		writeDebugLog("source=%s event=%s suppressed_by_config=true", e.Source, e.Type)
		return nil
	}
	if debounce.Suppressed(homeDir, e, time.Now()) {
		writeDebugLog("source=%s event=%s suppressed_by_debounce=true", e.Source, e.Type)
		return nil
	}
	_ = os.Setenv("AGENTBELL_TERMINAL", cfg.Terminal.App)
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

func doctor(args []string, stdout io.Writer) error {
	fix := len(args) == 1 && args[0] == "--fix"
	if len(args) > 0 && !fix {
		return fmt.Errorf("unknown doctor option %q", args[0])
	}
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(stdout, "AgentBell Doctor\n\n✗ macOS is required\n\nStatus\n✗ Some checks need attention")
		return doctorExit{code: 2}
	}
	homeDir, _ := os.UserHomeDir()
	executable, _ := os.Executable()
	claudeHooks, claudeMissing, claudeErr := install.HasAgentBellHooks(filepath.Join(homeDir, ".claude", "settings.json"), []string{"Notification", "PermissionRequest", "Stop", "StopFailure"})
	codexHooks, codexMissing, codexErr := install.HasAgentBellHooks(filepath.Join(homeDir, ".codex", "hooks.json"), []string{"PermissionRequest", "Stop"})
	configPath := config.Path(homeDir)
	_, configErr := config.Load(configPath)
	if fix {
		if err := notify.RegisterNativeApp(notify.NativeHelperFor(executable)); err != nil {
			return fmt.Errorf("repair native notification app: %w", err)
		}
	}
	notificationStatus := notificationPermission(executable)
	if fix && (!claudeHooks || !codexHooks) {
		claudeOwned, _ := install.HasAnyAgentBellHooks(filepath.Join(homeDir, ".claude", "settings.json"))
		codexOwned, _ := install.HasAnyAgentBellHooks(filepath.Join(homeDir, ".codex", "hooks.json"))
		if claudeOwned || codexOwned {
			fmt.Fprintln(stdout, "Repairing existing AgentBell hook registrations...")
			if err := repairHooks(homeDir, executable, claudeOwned, codexOwned); err != nil {
				fmt.Fprintf(stdout, "Repair failed: %v\n", err)
			}
		} else {
			fmt.Fprintln(stdout, "No existing AgentBell hook installation found; run agentbell install to set it up.")
		}
		claudeHooks, claudeMissing, claudeErr = install.HasAgentBellHooks(filepath.Join(homeDir, ".claude", "settings.json"), []string{"Notification", "PermissionRequest", "Stop", "StopFailure"})
		codexHooks, codexMissing, codexErr = install.HasAgentBellHooks(filepath.Join(homeDir, ".codex", "hooks.json"), []string{"PermissionRequest", "Stop"})
	}
	checks := []struct {
		name string
		ok   bool
		info string
	}{
		{"macOS", runtime.GOOS == "darwin", runtime.GOOS},
		{"Native helper", notify.NativeHelperFor(executable) != "", notify.NativeHelperFor(executable)},
		{"Notifications", notificationStatus == "authorized", notificationPermissionLabel(notificationStatus)},
		{"Claude Code", commandAvailable("claude"), commandVersion("claude")},
		{"Codex", commandAvailable("codex"), commandVersion("codex")},
		{"Claude hooks", claudeErr == nil && claudeHooks, doctorHookInfo(claudeMissing, claudeErr)},
		{"Codex hooks", codexErr == nil && codexHooks, doctorHookInfo(codexMissing, codexErr)},
		{"Config", configErr == nil, doctorConfigInfo(configPath, configErr)},
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
	return doctorExit{code: 1}
}

func notificationPermission(executable string) string {
	helper := notify.NativeHelperFor(executable)
	if helper == "" {
		return "unavailable"
	}
	out, err := exec.Command(helper, "--check").CombinedOutput()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(out))
}

func notificationPermissionInfo(executable string) string {
	return notificationPermissionLabel(notificationPermission(executable))
}

func notificationPermissionLabel(status string) string {
	switch status {
	case "authorized":
		return "authorized"
	case "denied":
		return "denied in System Settings"
	case "notDetermined":
		return "not requested yet (run agentbell test)"
	default:
		return "unavailable"
	}
}

type doctorExit struct{ code int }

func (e doctorExit) Error() string { return fmt.Sprintf("doctor failed with exit code %d", e.code) }

func doctorConfigInfo(path string, err error) string {
	if err != nil {
		return "invalid " + path + ": " + err.Error()
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return "defaults (optional config missing)"
	}
	return path
}

func repairHooks(home, executable string, claude, codex bool) error {
	if notify.NativeHelperFor(executable) == "" {
		return errors.New("native helper is missing; reinstall AgentBell package")
	}
	installer := install.New(home, executable)
	if claude {
		if err := installer.InstallClaude(); err != nil {
			return err
		}
	}
	if codex {
		if err := installer.InstallCodex(); err != nil {
			return err
		}
	}
	return nil
}

func installPreview(stdout io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	preview, err := install.Preview(home)
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, preview)
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
  agentbell doctor --fix
  agentbell version
  agentbell install
  agentbell install --dry-run
  agentbell uninstall
`
	fmt.Fprint(w, usage)
}
