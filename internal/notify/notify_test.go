package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/surface"
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

func TestNativeHelperReceivesReturnTargetAndAction(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("test"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	t.Setenv("AGENTBELL_TERMINAL", "iterm2")
	var gotName string
	var gotArgs []string
	sender := MacOS{Run: func(name string, args ...string) ([]byte, error) { gotName = name; gotArgs = args; return nil, nil }}
	err := sender.Send(event.AgentEvent{Source: "claude", Type: event.NeedsApproval, CWD: "/tmp/a path", Project: "demo", SessionID: "session-click", ReturnTarget: &surface.ReturnTarget{Surface: "tabby", AppName: "Tabby", AppBundleID: "org.tabby", CWD: "/tmp/a path", Capability: surface.ReturnApp}})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != helper || len(gotArgs) != 9 || !strings.Contains(gotArgs[3], `"AppBundleID":"org.tabby"`) || gotArgs[4] != "打开 Tabby" {
		t.Fatalf("unexpected helper args: %q %q", gotName, gotArgs)
	}
	if gotArgs[6] != "claude" || gotArgs[7] != "needs_approval" || gotArgs[8] != "session-click" {
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

func TestNativeNotificationFiltersEmptySuggestions(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	for _, tc := range []struct{ name, input, want string }{
		{"empty suggestions", `{"suggestions":[]}`, "Task completed."},
		{"formatted empty suggestions", " \n{\"suggestions\" : [\n]}\n", "Task completed."},
		{"heartbeat empty suggestions", `<heartbeat><message>{"suggestions":[]}</message></heartbeat>`, "Task completed."},
		{"nonempty suggestions", `{"suggestions":["Run tests"]}`, `{"suggestions":["Run tests"]}`},
		{"other useful fields", `{"suggestions":[],"message":"Tests passed"}`, `{"suggestions":[],"message":"Tests passed"}`},
		{"null is not an empty array", `{"suggestions":null}`, `{"suggestions":null}`},
		{"ordinary JSON", `{"files":[]}`, `{"files":[]}`},
		{"malformed JSON", `{"suggestions":[}`, `{"suggestions":[}`},
		{"ordinary summary", "已修复问题\n测试通过", "已修复问题 测试通过"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
}

func TestPermissionRequestUsesNeutralCopyAndReturnCTA(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("helper"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	var args []string
	sender := MacOS{Run: func(_ string, a ...string) ([]byte, error) { args = a; return nil, nil }}
	target := &surface.ReturnTarget{Surface: "tabby", AppName: "Tabby", AppBundleID: "org.tabby", Capability: surface.ReturnApp}
	err := sender.Send(event.AgentEvent{Source: "codex", Type: event.EventPermissionRequest, Project: "demo", Message: "Approval needed", ReturnTarget: target})
	if err != nil {
		t.Fatal(err)
	}
	if args[1] != "Permission requested" || strings.Contains(strings.Join(args, " "), "Approval needed") {
		t.Fatalf("misleading permission copy: %v", args)
	}
	if args[3] != surface.Encode(target) || args[4] != surface.ActionTitle(*target) || args[7] != "permission_request" {
		t.Fatalf("return CTA/event lost: %v", args)
	}
}

func TestResidentContentMatchesFallbackAndPreservesReturn(t *testing.T) {
	target := &surface.ReturnTarget{Capability: "exact_context"}
	e := event.AgentEvent{Source: "claude", Type: event.Done, Project: "demo", SessionID: "s1", Message: `{"suggestions":[]}`, ReturnTarget: target}
	c := ContentFor(e)
	if c.Kind != "notification" || c.Title != "Claude · demo" || c.Body != "Task completed." || c.Subtitle != "Task completed" || c.SessionID != "s1" || c.Target != surface.Encode(target) || c.Action != surface.ActionTitle(*target) {
		t.Fatalf("resident notification lost formatting or return metadata: %+v", c)
	}
}

func TestNotificationPrefersSessionTitleOverProject(t *testing.T) {
	e := event.AgentEvent{Source: "codex", Type: event.Done, Project: "Toy", SessionTitle: "Build AgentBell v0.4 Attention"}
	if got := ContentFor(e).Title; got != "Codex · Build AgentBell v0.4 Attention" {
		t.Fatal(got)
	}
	e.SessionTitle = ""
	if got := ContentFor(e).Title; got != "Codex · Toy" {
		t.Fatal("project fallback lost", got)
	}
}
