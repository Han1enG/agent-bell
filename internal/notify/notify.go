package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/han1eng/agent-bell/internal/event"
)

type Sender interface {
	Send(event.AgentEvent) error
}

type MacOS struct {
	Run func(name string, args ...string) ([]byte, error)
}

func (n MacOS) Send(e event.AgentEvent) error {
	if n.Run == nil {
		n.Run = func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).CombinedOutput()
		}
	}
	e.Normalize()
	text := e.Message
	if text == "" {
		text = defaultMessage(e)
	}
	text = compactSummary(text)
	title := e.Project
	if title == "" {
		title = e.Title
	}
	subtitle := statusLabel(e.Type)
	script := fmt.Sprintf("const app = Application.currentApplication(); app.includeStandardAdditions = true; app.displayNotification(%s, {withTitle: %s, subtitle: %s});", jsString(text), jsString(title), jsString(subtitle))
	command, args := notificationCommand(title, subtitle, text, script)
	output, err := n.Run(command, args...)
	if err != nil {
		return fmt.Errorf("send macOS notification: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func notificationCommand(title, subtitle, text, fallbackScript string) (string, []string) {
	if helper := nativeHelperPath(); helper != "" {
		return helper, []string{title, subtitle, text}
	}
	return "osascript", []string{"-l", "JavaScript", "-e", fallbackScript}
}

func nativeHelperPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return NativeHelperFor(executable)
}

// NativeHelperFor returns the bundled macOS notification helper for an
// AgentBell executable path. It is also used by install and doctor.
func NativeHelperFor(executable string) string {
	if configured := os.Getenv("AGENTBELL_NOTIFIER"); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}
	if executable == "" {
		return ""
	}
	dir := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(dir, "AgentBellNotifier"),
		filepath.Join(dir, "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier"),
		filepath.Join(dir, "..", "Applications", "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

func compactSummary(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 180 {
		return string(runes[:177]) + "..."
	}
	return value
}

func defaultMessage(e event.AgentEvent) string {
	switch e.Type {
	case event.Done:
		return "Task completed."
	case event.NeedsApproval:
		return fmt.Sprintf("%s needs your permission.", e.Title)
	case event.NeedsInput:
		return fmt.Sprintf("%s is waiting for your input.", e.Title)
	case event.Error:
		return fmt.Sprintf("%s task failed.", e.Title)
	default:
		return "Agent event received."
	}
}

func statusLabel(t event.Type) string {
	switch t {
	case event.Done:
		return "Task completed"
	case event.NeedsApproval:
		return "Needs approval"
	case event.NeedsInput:
		return "Needs input"
	case event.Error:
		return "Task failed"
	default:
		return "Agent event"
	}
}

func jsString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}
