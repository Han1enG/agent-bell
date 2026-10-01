package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/event"
)

func TestMacOSSendBuildsSafeScript(t *testing.T) {
	var name string
	var args []string
	sender := MacOS{Run: func(command string, commandArgs ...string) ([]byte, error) {
		name = command
		args = commandArgs
		return nil, nil
	}}

	err := sender.Send(event.AgentEvent{
		Source:  "claude",
		Type:    event.Done,
		Project: "demo",
		Title:   "Claude Code",
		Message: `finished "now"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "osascript" || len(args) != 4 || args[0] != "-l" || args[1] != "JavaScript" || args[2] != "-e" {
		t.Fatalf("unexpected command: %q %q", name, args)
	}
	if !strings.Contains(args[3], `finished \"now\"`) || !strings.Contains(args[3], `demo`) || !strings.Contains(args[3], `Task completed`) {
		t.Fatalf("message was not escaped: %s", args[3])
	}
}

func TestNativeHelperForResolvesHomebrewSymlink(t *testing.T) {
	root := t.TempDir()
	cellarBin := filepath.Join(root, "Cellar", "agentbell", "0.1.0", "bin")
	helper := filepath.Join(root, "Cellar", "agentbell", "0.1.0", "libexec", "AgentBell.app", "Contents", "MacOS", "AgentBellNotifier")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, "bin", "agentbell")), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cellarBin, "agentbell")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("agentbell"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "bin", "agentbell")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(helper)
	if err != nil {
		t.Fatal(err)
	}
	if got := NativeHelperFor(link); got != want {
		t.Fatalf("NativeHelperFor(%q) = %q, want %q", link, got, want)
	}
}

func TestNativeHelperReceivesCWDAndTerminalAsArguments(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("test"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	t.Setenv("AGENTBELL_TERMINAL", "iterm2")
	var gotName string
	var gotArgs []string
	sender := MacOS{Run: func(name string, args ...string) ([]byte, error) { gotName = name; gotArgs = args; return nil, nil }}
	err := sender.Send(event.AgentEvent{Source: "claude", Type: event.NeedsApproval, CWD: "/tmp/a path", Project: "demo", SessionID: "session-click"})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != helper || len(gotArgs) != 8 || gotArgs[3] != "/tmp/a path" || gotArgs[4] != "iterm2" {
		t.Fatalf("unexpected helper args: %q %q", gotName, gotArgs)
	}
	if gotArgs[5] != "claude" || gotArgs[6] != "needs_approval" || gotArgs[7] != "session-click" {
		t.Fatalf("notification lost click context: %q", gotArgs)
	}
}
