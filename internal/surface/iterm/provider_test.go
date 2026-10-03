package iterm

import (
	"errors"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

func TestExactIdentityAndRestart(t *testing.T) {
	started := "Fri Oct 2 12:00:00 2026"
	present := true
	var modes []string
	p := Provider{Run: func(name string, a ...string) ([]byte, error) {
		if name == "/bin/ps" {
			if a[1] == "42" {
				return []byte("77 ttys007 Fri Oct 2 12:00:00 2026 /bin/zsh"), nil
			}
			return []byte("1 ?? " + started + " /Applications/iTerm.app/Contents/MacOS/iTerm2"), nil
		}
		if name == "/usr/bin/osascript" {
			modes = append(modes, a[len(a)-1])
			if !present {
				return []byte("context_not_found"), errors.New("expired")
			}
			return []byte("1:0"), nil
		}
		return nil, errors.New("unexpected")
	}}
	target, err := p.Detect(surface.DetectContext{PID: 42, Env: func(k string) string {
		if k == "ITERM_SESSION_ID" {
			return "w0t0p0:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		}
		return ""
	}})
	if err != nil || target.Capability != surface.ReturnExactContext || len(modes) != 0 {
		t.Fatal(target, err, modes)
	}
	if err := p.Return(*target); err != nil || strings.Join(modes, ",") != "probe,focus" {
		t.Fatal(err, modes)
	}
	modes = nil
	present = false
	if err := p.Return(*target); surface.Reason(err) != surface.ContextNotFound || strings.Join(modes, ",") != "probe" {
		t.Fatal(err, modes)
	}
	modes = nil
	present = true
	started = "Fri Oct 2 12:10:00 2026"
	if err := p.Return(*target); surface.Reason(err) != surface.ContextNotFound || len(modes) > 0 {
		t.Fatal(err, modes)
	}
}
func TestScriptsOnlyFocusSelectAndReadIdentity(t *testing.T) {
	for _, bad := range []string{"write text", "contents of", "create tab", "split ", "close ", "do shell script"} {
		if strings.Contains(script, bad) {
			t.Fatal(bad)
		}
	}
	if strings.Index(script, "if item 3 of argv is \"probe\"") > strings.Index(script, "activate\n") {
		t.Fatal("probe activates")
	}
}
