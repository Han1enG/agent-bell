package wezterm

import (
	"errors"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

func TestPinnedInstanceAndStalePane(t *testing.T) {
	stamp := "g1"
	panes := `[{"pane_id":13,"tab_id":2,"window_id":3,"tty_name":"/dev/ttys007","is_active":true},{"pane_id":14,"tab_id":4,"window_id":3,"tty_name":"/dev/ttys007","is_active":true}]`
	var calls [][]string
	p := Provider{NativeHelper: "/test/helper", SocketIdentity: func(string) (string, error) { return stamp, nil }, Run: func(name string, a ...string) ([]byte, error) {
		if name == "/bin/ps" {
			return []byte("Fri Oct 2 12:00:00 2026 /Applications/WezTerm.app/Contents/MacOS/wezterm-gui"), nil
		}
		if name == "/test/helper" {
			if a[0] != "--activate-wezterm" || a[1] != "99" {
				t.Fatal(a)
			}
			return nil, nil
		}
		calls = append(calls, a)
		if name != "/usr/bin/env" || a[0] != "WEZTERM_UNIX_SOCKET=/tmp/gui-sock-99" || a[3] != "--no-auto-start" {
			t.Fatal(name, a)
		}
		if a[4] == "list" {
			return []byte(panes), nil
		}
		if a[4] == "activate-pane" && a[5] == "--pane-id" && a[6] == "13" {
			return nil, nil
		}
		return nil, errors.New("unexpected")
	}}
	target, err := p.Detect(surface.DetectContext{Env: func(k string) string {
		switch k {
		case "WEZTERM_PANE":
			return "13"
		case "WEZTERM_UNIX_SOCKET":
			return "/tmp/gui-sock-99"
		}
		return ""
	}})
	if err != nil || target == nil || len(calls) != 0 {
		t.Fatal(target, err, calls)
	}
	if err := p.VerifyClient(*target, "/dev/ttys007"); err != nil {
		t.Fatal(err)
	}
	if err := p.VerifyClient(*target, "/dev/ttys008"); surface.Reason(err) != surface.InstanceMismatch {
		t.Fatal(err)
	}
	if err := p.Return(*target); err != nil {
		t.Fatal(err)
	}
	calls = nil
	panes = `[{"pane_id":14,"tab_id":2,"window_id":3,"cwd":"same-project"}]`
	if err := p.Return(*target); surface.Reason(err) != surface.PaneNotFound {
		t.Fatal(err)
	}
	for _, a := range calls {
		if a[4] != "list" {
			t.Fatal(a)
		}
	}
	calls = nil
	stamp = "g2"
	if err := p.Return(*target); surface.Reason(err) != surface.InstanceMismatch || len(calls) > 0 {
		t.Fatal(err, calls)
	}
}
func TestNoSocketDoesNotGuessInstance(t *testing.T) {
	p := Provider{SocketIdentity: func(string) (string, error) { return "", errors.New("missing") }, Run: func(string, ...string) ([]byte, error) { t.Fatal("CLI"); return nil, nil }}
	target, _ := p.Detect(surface.DetectContext{Env: func(k string) string {
		if k == "WEZTERM_PANE" {
			return "0"
		}
		return ""
	}})
	if target.Capability != surface.ReturnApp {
		t.Fatal(target)
	}
	for _, id := range []string{`{"Socket":"/tmp/s","SocketIdentity":"g"}`, strings.Repeat("x", 4097)} {
		if p.Probe(surface.ReturnTarget{Surface: "wezterm", ContextID: id, Capability: surface.ReturnExactContext}) == nil {
			t.Fatal(id)
		}
	}
}
