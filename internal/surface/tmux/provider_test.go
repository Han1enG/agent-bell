package tmux

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

type fixture struct {
	stamp, rows, clients string
	calls                [][]string
	selected             string
	unreachable          bool
}

func (f *fixture) provider() Provider {
	return Provider{SocketIdentity: func(string) (string, error) { return f.stamp, nil }, Run: func(_ string, a ...string) ([]byte, error) {
		f.calls = append(f.calls, append([]string{}, a...))
		if f.unreachable {
			return nil, errors.New("unreachable")
		}
		switch a[2] {
		case "list-panes":
			return []byte(f.rows), nil
		case "list-clients":
			return []byte(f.clients), nil
		case "display-message":
			if len(a) > 5 && a[4] == "-c" {
				return []byte(f.selected), nil
			}
			return []byte(strings.Split(f.rows, "\n")[0]), nil
		case "select-window":
			return nil, nil
		case "select-pane":
			f.selected = a[len(a)-1]
			return nil, nil
		}
		return nil, errors.New("unexpected command")
	}}
}
func newFixture() *fixture {
	return &fixture{stamp: "generation-1", rows: "%7\t@2\t$1\t123\t1700000000", clients: "/dev/ttys007\t456\t1700000010\t$1\t0", selected: "%8"}
}
func detect(t *testing.T, f *fixture) surface.ReturnTarget {
	t.Helper()
	p := f.provider()
	target, err := p.Detect(surface.DetectContext{Env: func(k string) string {
		switch k {
		case "TMUX":
			return "/tmp/custom,socket,123,1"
		case "TMUX_PANE":
			return "%7"
		}
		return ""
	}})
	if err != nil || target == nil {
		t.Fatal(target, err)
	}
	return *target
}
func TestDetectCustomSocketAndExactFocus(t *testing.T) {
	f := newFixture()
	target := detect(t, f)
	var c Context
	json.Unmarshal([]byte(target.ContextID), &c)
	if c.ServerSocket != "/tmp/custom,socket" || target.Capability != surface.ReturnExactContext {
		t.Fatal(c, target)
	}
	f.calls = nil
	if err := f.provider().Return(target); err != nil {
		t.Fatal(err)
	}
	if f.selected != "%7" {
		t.Fatal(f.selected)
	}
	for _, a := range f.calls {
		switch a[2] {
		case "display-message", "list-clients", "list-panes", "select-window", "select-pane":
		default:
			t.Fatal("unsafe", a)
		}
		if a[0] != "-S" || a[1] != c.ServerSocket {
			t.Fatal(a)
		}
	}
}
func TestStaleContextsNeverSelect(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*fixture)
		reason surface.FailureReason
	}{
		{"pane-close", func(f *fixture) { f.rows = "%8\t@2\t$1\t123\t1700000000" }, surface.PaneNotFound},
		{"window-close", func(f *fixture) { f.rows = "%7\t@3\t$1\t123\t1700000000" }, surface.ContextNotFound},
		{"session-close", func(f *fixture) { f.rows = "%7\t@2\t$2\t123\t1700000000" }, surface.ContextNotFound},
		{"server-restart-reused-id", func(f *fixture) { f.stamp = "generation-2" }, surface.ServerIdentityMismatch},
		{"server-pid-reuse", func(f *fixture) { f.rows = "%7\t@2\t$1\t123\t1700000100" }, surface.ServerIdentityMismatch},
		{"client-detached", func(f *fixture) { f.clients = "" }, surface.ContextNotFound},
		{"client-reattached", func(f *fixture) { f.clients = "/dev/ttys007\t456\t1700000200\t$1\t0" }, surface.ContextNotFound},
		{"multiple-clients", func(f *fixture) { f.clients += "\n/dev/ttys008\t457\t1700000020\t$1\t0" }, surface.ContextNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			target := detect(t, f)
			tc.change(f)
			f.calls = nil
			err := f.provider().Return(target)
			if surface.Reason(err) != tc.reason {
				t.Fatal(err)
			}
			for _, a := range f.calls {
				if strings.HasPrefix(a[2], "select-") {
					t.Fatal("stale target selected", a)
				}
			}
		})
	}
}
func TestProbeReadOnlyMultiWindowSameCWD(t *testing.T) {
	f := newFixture()
	f.rows += "\n%8\t@2\t$1\t123\t1700000000\n%9\t@3\t$1\t123\t1700000000"
	target := detect(t, f)
	f.calls = nil
	if err := f.provider().Probe(target); err != nil {
		t.Fatal(err)
	}
	if f.selected != "%8" || !reflect.DeepEqual([]string{f.calls[0][2], f.calls[1][2]}, []string{"list-panes", "list-clients"}) {
		t.Fatal(f.calls)
	}
}
func TestNoEnvironmentNoCLI(t *testing.T) {
	p := Provider{Run: func(string, ...string) ([]byte, error) { t.Fatal("CLI called"); return nil, nil }}
	if target, err := p.Detect(surface.DetectContext{Env: func(string) string { return "" }}); target != nil || err != nil {
		t.Fatal(target, err)
	}
}
func TestAmbiguousClientDoesNotClaimExact(t *testing.T) {
	f := newFixture()
	f.clients += "\n/dev/ttys008\t457\t1700000020\t$1\t0"
	target := detect(t, f)
	if target.Capability == surface.ReturnExactContext {
		t.Fatal(target)
	}
}
