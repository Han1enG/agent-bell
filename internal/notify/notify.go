package notify

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	text := compactSummary(e.Message)
	if text == "" {
		text = defaultMessage(e)
	}
	title := e.Project
	if title == "" {
		title = e.Title
	}
	subtitle := statusLabel(e.Type)
	script := fmt.Sprintf("const app = Application.currentApplication(); app.includeStandardAdditions = true; app.displayNotification(%s, {withTitle: %s, subtitle: %s});", jsString(text), jsString(title), jsString(subtitle))
	command, args := notificationCommand(title, subtitle, text, script)
	if command != "osascript" {
		terminal := strings.ToLower(strings.TrimSpace(os.Getenv("AGENTBELL_TERMINAL")))
		if terminal != "iterm2" {
			terminal = "terminal"
		}
		args = append(args, e.CWD, terminal, e.Source, string(e.Type), e.SessionID)
	}
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
	executablePaths := []string{executable}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != executable {
		executablePaths = append(executablePaths, resolved)
	}
	for _, executablePath := range executablePaths {
		dir := filepath.Dir(executablePath)
		candidates := []string{
			filepath.Join(dir, "AgentBellNotifier"),
			filepath.Join(dir, "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier"),
			filepath.Join(dir, "..", "libexec", "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier"),
			filepath.Join(dir, "..", "Applications", "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier"),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && info.Mode()&0o111 != 0 {
				return candidate
			}
		}
	}
	return ""
}

// RegisterNativeApp registers the installed bundle's notification click entry.
func RegisterNativeApp(helper string) error {
	if runtime.GOOS != "darwin" || helper == "" {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(helper)
	if err != nil {
		return err
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(resolved)))
	if filepath.Ext(app) != ".app" || filepath.Base(filepath.Dir(resolved)) != "MacOS" {
		return nil // Standalone helpers and test fixtures have no bundle.
	}
	registrar := "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	if output, err := exec.Command(registrar, "-f", app).CombinedOutput(); err != nil {
		return fmt.Errorf("register notification app: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func compactSummary(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "<heartbeat>") || strings.HasPrefix(value, "<heartbeat ") || strings.HasPrefix(value, "<heartbeat\n") {
		var heartbeat struct {
			XMLName xml.Name `xml:"heartbeat"`
			Message string   `xml:"message"`
		}
		if err := xml.Unmarshal([]byte(value), &heartbeat); err != nil {
			// A broken control envelope is not useful notification text. Let
			// Send use the event's default instead of exposing protocol fields.
			value = ""
		} else {
			value = heartbeat.Message
		}
	}
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
		return fmt.Sprintf("%s requested permission. Check %s to see whether it still needs your input.", e.Title, e.Title)
	case event.NeedsInput:
		return "Waiting for your input. Click to open the project."
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
		return "Permission request"
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
