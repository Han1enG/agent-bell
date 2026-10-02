package terminal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/notify"
	"github.com/han1eng/agent-bell/internal/surface"
)

func terminalEnv(key string) string {
	if key == "TERM_PROGRAM" {
		return "Apple_Terminal"
	}
	return ""
}
func TestProcessIdentityIsIndependentOfLocale(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS ps format")
	}
	p := Provider{}
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	parent, tty, started, err := p.process(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse("Mon Jan 2 15:04:05 2006", started); err != nil {
		t.Fatal("process identity was localized", started, err)
	}
	t.Setenv("LC_ALL", "C")
	otherParent, otherTTY, otherStarted, err := p.process(os.Getpid())
	if err != nil || parent != otherParent || tty != otherTTY || started != otherStarted {
		t.Fatal("process identity changed with locale", started, otherStarted, err)
	}
}
func TestDetectRecordsPersistentSession(t *testing.T) {
	var appleEvents bool
	p := Provider{Run: func(name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/osascript" {
			appleEvents = true
			t.Fatal("hook detection sent an Apple event")
		}
		if len(args) == 6 {
			return []byte("1 /bin/zsh\n"), nil
		} // Generic ancestry
		switch args[1] {
		case "100":
			return []byte("90 ttys001 Fri Oct 2 15:00:00 2026"), nil
		case "90":
			return []byte("80 ttys001 Fri Oct 2 14:00:00 2026"), nil
		case "80":
			return []byte("1 ?? Fri Oct 2 13:00:00 2026"), nil
		}
		return nil, errors.New("missing")
	}}
	target, err := p.Detect(surface.DetectContext{Env: terminalEnv, PID: 100, CWD: "/same", AgentSessionID: "agent"})
	if err != nil || target == nil || target.Capability != surface.ReturnExactContext || appleEvents {
		t.Fatal(target, err)
	}
	identity, err := parseContext(target.ContextID)
	if err != nil || identity.PID != 90 || identity.TTY != "/dev/ttys001" || target.AgentSessionID != "agent" {
		t.Fatal(identity, err)
	}
}
func TestDetectSkipsTmuxAndMissingTTY(t *testing.T) {
	p := Provider{Run: func(string, ...string) ([]byte, error) { return nil, errors.New("no process") }}
	for _, tmux := range []bool{false, true} {
		target, err := p.Detect(surface.DetectContext{Env: func(key string) string {
			if key == "TMUX" && tmux {
				return "tmux"
			}
			return terminalEnv(key)
		}})
		if err != nil || target.Capability != surface.ReturnApp || target.ContextID != "" {
			t.Fatal(target, err)
		}
	}
	target, err := p.Detect(surface.DetectContext{Env: func(key string) string {
		if key == "AGENTBELL_SURFACE" {
			return "tabby"
		}
		return terminalEnv(key)
	}})
	if err != nil || target != nil {
		t.Fatal("overrode explicit surface", target, err)
	}
}
func targetFor(c Context) surface.ReturnTarget {
	data, _ := json.Marshal(c)
	return surface.ReturnTarget{Surface: "terminal", AppBundleID: "com.apple.Terminal", ContextID: string(data), Capability: surface.ReturnExactContext}
}
func TestFocusOnlyValidExistingSession(t *testing.T) {
	identity := Context{TTY: "/dev/ttys001", PID: 90, Started: "Fri Oct 2 14:00:00 2026"}
	var calls []string
	p := Provider{Run: func(name string, args ...string) ([]byte, error) {
		calls = append(calls, name)
		if name == "/bin/ps" {
			return []byte("80 ttys001 Fri Oct 2 14:00:00 2026"), nil
		}
		if !reflect.DeepEqual(args, []string{"-e", focusScript, "--", "/dev/ttys001"}) {
			t.Fatal(args)
		}
		if strings.Contains(focusScript, "do script") || strings.Contains(focusScript, "do shell script") {
			t.Fatal("focus executes commands")
		}
		return []byte("123"), nil
	}}
	if err := p.Return(targetFor(identity)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"/bin/ps", "/usr/bin/osascript"}) {
		t.Fatal(calls)
	}
}
func TestExpiredSessionNeverSendsAppleEvents(t *testing.T) {
	for _, result := range []string{"80 ttys002 Fri Oct 2 14:00:00 2026", "80 ttys001 Fri Oct 2 15:00:00 2026", ""} {
		p := Provider{Run: func(name string, _ ...string) ([]byte, error) {
			if name != "/bin/ps" {
				t.Fatal("focused reused session")
			}
			return []byte(result), nil
		}}
		if p.Return(targetFor(Context{TTY: "/dev/ttys001", PID: 90, Started: "Fri Oct 2 14:00:00 2026"})) == nil {
			t.Fatal("accepted expired session")
		}
	}
}
func TestInvalidContextAndPermissionFailureFallback(t *testing.T) {
	p := Provider{Run: func(string, ...string) ([]byte, error) { t.Fatal("invalid context executed command"); return nil, nil }}
	for _, bad := range []Context{{TTY: `/dev/ttys001"; bad`, PID: 90, Started: "Fri Oct 2 14:00:00 2026"}, {TTY: "/dev/ttys001", PID: 0, Started: "Fri Oct 2 14:00:00 2026"}} {
		if p.Return(targetFor(bad)) == nil {
			t.Fatal("invalid context accepted")
		}
	}
	var activated bool
	p.Run = func(name string, _ ...string) ([]byte, error) {
		if name == "/bin/ps" {
			return []byte("80 ttys001 Fri Oct 2 14:00:00 2026"), nil
		}
		return []byte("not authorized -1743"), errors.New("denied")
	}
	generic := surface.GenericProvider{Run: func(name string, args ...string) ([]byte, error) {
		if name != "/usr/bin/open" || !reflect.DeepEqual(args, []string{"-b", "com.apple.Terminal"}) {
			t.Fatal(name, args)
		}
		activated = true
		return nil, nil
	}}
	if err := (surface.Manager{Providers: []surface.SurfaceProvider{p, generic}}).ReturnToContext(targetFor(Context{TTY: "/dev/ttys001", PID: 90, Started: "Fri Oct 2 14:00:00 2026"})); err != nil || !activated {
		t.Fatal(err, activated)
	}
}

func TestProbeChecksTabWithoutFocus(t *testing.T) {
	calls := 0
	p := Provider{Run: func(name string, args ...string) ([]byte, error) {
		calls++
		if name == "/bin/ps" {
			return []byte("80 ttys001 Fri Oct 2 14:00:00 2026"), nil
		}
		if args[len(args)-1] != "probe" {
			t.Fatal("probe invoked focus", args)
		}
		if strings.Index(focusScript, `then return targetWindowID as text`) > strings.Index(focusScript, "set selected") {
			t.Fatal("probe mutates UI")
		}
		return []byte("12"), nil
	}}
	if err := p.Probe(targetFor(Context{TTY: "/dev/ttys001", PID: 90, Started: "Fri Oct 2 14:00:00 2026"})); err != nil || calls != 2 {
		t.Fatal(err, calls)
	}
}

func TestNotificationIsIndependentOfAutomationDenial(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	target := targetFor(Context{TTY: "/dev/ttys001", PID: 90, Started: "Fri Oct 2 14:00:00 2026"})
	posted := false
	sender := notify.MacOS{Run: func(name string, _ ...string) ([]byte, error) {
		if name != helper {
			t.Fatal("notification invoked Automation", name)
		}
		posted = true
		return nil, nil
	}}
	if err := sender.Send(event.AgentEvent{Source: "codex", Type: event.Done, Project: "fixture", ReturnTarget: &target}); err != nil || !posted {
		t.Fatal(err)
	}
	p := Provider{Run: func(name string, _ ...string) ([]byte, error) {
		if name == "/bin/ps" {
			return []byte("80 ttys001 Fri Oct 2 14:00:00 2026"), nil
		}
		return []byte("not authorized -1743"), errors.New("denied")
	}}
	if reason := surface.Reason(p.Probe(target)); reason != surface.PermissionDenied {
		t.Fatal(reason)
	}
}
