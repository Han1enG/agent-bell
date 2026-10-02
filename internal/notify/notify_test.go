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

func TestCompactSummaryExtractsHeartbeatMessage(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"heartbeat", `<heartbeat><automation_id>automation</automation_id><decision>NOTIFY</decision><message>你好</message></heartbeat>`, "你好"},
		{"multiline and entities", " <heartbeat>\n<decision>NOTIFY</decision><message>构建完成 &amp; 测试通过\n可以查看 PR</message></heartbeat> ", "构建完成 & 测试通过 可以查看 PR"},
		{"CDATA", `<heartbeat><message><![CDATA[结果包含 <example>]]></message></heartbeat>`, "结果包含 <example>"},
		{"long metadata", `<heartbeat><automation_id>` + strings.Repeat("a", 300) + `</automation_id><message>有新结果</message></heartbeat>`, "有新结果"},
		{"empty message", `<heartbeat><decision>NOTIFY</decision><message></message></heartbeat>`, ""},
		{"broken envelope", `<heartbeat><automation_id>internal</automation_id><message>broken`, ""},
		{"ordinary text", "任务完成\n 修改了 3 个文件", "任务完成 修改了 3 个文件"},
		{"ordinary XML", `<example>keep this code</example>`, `<example>keep this code</example>`},
		{"truncate message", `<heartbeat><message>` + strings.Repeat("好", 181) + `</message></heartbeat>`, strings.Repeat("好", 177) + "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compactSummary(tc.input); got != tc.want {
				t.Fatalf("summary = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNativeNotificationDoesNotExposeHeartbeatEnvelope(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	for _, tc := range []struct{ input, want string }{
		{`<heartbeat><automation_id>automation</automation_id><decision>NOTIFY</decision><message>你好</message></heartbeat>`, "你好"},
		{`<heartbeat><message></message></heartbeat>`, "Task completed."},
	} {
		var body string
		sender := MacOS{Run: func(_ string, args ...string) ([]byte, error) {
			body = args[2]
			return nil, nil
		}}
		if err := sender.Send(event.AgentEvent{Source: "codex", Type: event.Done, Message: tc.input}); err != nil {
			t.Fatal(err)
		}
		if body != tc.want {
			t.Fatalf("notification body = %q, want %q", body, tc.want)
		}
	}
}
