package notify

import (
	"strings"
	"testing"

	"github.com/agentbell/agentbell/internal/event"
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
