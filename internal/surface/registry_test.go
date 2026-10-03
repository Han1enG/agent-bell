package surface

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type stackProvider struct {
	name                     string
	calls                    *[]string
	valid, bound, panicProbe bool
	detect                   *ReturnTarget
}

func (p stackProvider) Name() string                                { return p.name }
func (p stackProvider) Detect(DetectContext) (*ReturnTarget, error) { return p.detect, nil }
func (p stackProvider) CanHandle(t ReturnTarget) bool {
	return t.Surface == p.name && t.Capability == ReturnExactContext
}
func (p stackProvider) Probe(ReturnTarget) error {
	*p.calls = append(*p.calls, p.name+" probe")
	if p.panicProbe {
		panic("fixture")
	}
	if !p.valid {
		return Fail(ContextNotFound, errors.New("expired"))
	}
	return nil
}
func (p stackProvider) Return(ReturnTarget) error {
	*p.calls = append(*p.calls, p.name+" focus")
	return nil
}
func (p stackProvider) VerifyClient(_ ReturnTarget, tty string) error {
	if !p.bound || tty != "/dev/ttys007" {
		return Fail(InstanceMismatch, errors.New("wrong tty"))
	}
	return nil
}
func (p stackProvider) ClientTTY(ReturnTarget) (string, error) { return "/dev/ttys007", nil }
func TestCompositeOrderingStaleAndIsolation(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		innerValid, bound, crash, focus bool
	}{{"live", true, true, false, true}, {"stale", false, true, false, false}, {"wrong-tab", true, false, false, false}, {"panic", true, true, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			outer := stackProvider{name: "outer", calls: &calls, valid: true, bound: tc.bound}
			inner := stackProvider{name: "tmux", calls: &calls, valid: tc.innerValid, panicProbe: tc.crash}
			generic := GenericProvider{Run: func(string, ...string) ([]byte, error) { calls = append(calls, "activate app"); return nil, nil }}
			target := ReturnTarget{AppBundleID: "app", Capability: ReturnExactContext, Layers: []SurfaceLayer{{Provider: "outer", ContextID: "outer-id", Capability: ReturnExactContext}, {Provider: "tmux", ContextID: "inner-id", Metadata: map[string]string{"client_tty": "untrusted"}, Capability: ReturnExactContext}}}
			if err := (Manager{Providers: []SurfaceProvider{outer, inner, generic}}).ReturnToContext(target); err != nil {
				t.Fatal(err)
			}
			if calls[0] != "activate app" {
				t.Fatal(calls)
			}
			focused := false
			for i, c := range calls {
				if c == "tmux focus" {
					focused = true
					if i < 1 || calls[i-2] != "outer focus" {
						t.Fatal(calls)
					}
				}
			}
			if focused != tc.focus {
				t.Fatal(calls)
			}
			if !tc.focus {
				for _, c := range calls {
					if c == "outer focus" {
						t.Fatal("wrong outer selected", calls)
					}
				}
			}
		})
	}
}
func TestRegistryPriorityCompositeAndMetadataRoundtrip(t *testing.T) {
	var calls []string
	outer := ReturnTarget{Surface: "outer", ContextID: "opaque", AppBundleID: "app", Capability: ReturnExactContext}
	inner := ReturnTarget{Surface: "tmux", ContextID: "opaque-inner", Capability: ReturnExactContext, Metadata: map[string]string{"client_tty": "/dev/ttys007"}}
	var r Registry
	r.Register(GenericProvider{}, -100, false)
	r.Register(stackProvider{name: "outer", calls: &calls, valid: true, bound: true, detect: &outer}, 10, false)
	r.Register(stackProvider{name: "tmux", calls: &calls, valid: true, detect: &inner}, 20, true)
	target := r.Detect(DetectContext{Env: func(string) string { return "" }})
	if target.Surface != "outer" || len(target.Layers) != 2 || target.Capability != ReturnExactContext {
		t.Fatal(target)
	}
	if got := r.Providers()[len(r.Providers())-1].Name(); got != "generic" {
		t.Fatal(got)
	}
	plan := BuildPlan(*target)
	if len(plan.Steps) != 3 || !reflect.DeepEqual(plan.Steps[2].Target.Metadata, inner.Metadata) {
		t.Fatal(plan)
	}
}

func TestExpiredDetectionBudgetDoesNotLaunchCommands(t *testing.T) {
	runner := DetectionRunner(DetectContext{Deadline: time.Now().Add(-time.Second)}, nil)
	_, err := runner("/nonexistent/should-not-launch")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

type crashingHandler struct{ stackProvider }

func (crashingHandler) CanHandle(ReturnTarget) bool { panic("handler failed") }
func TestHandlerPanicCannotBlockAppFallback(t *testing.T) {
	var calls []string
	broken := crashingHandler{stackProvider{name: "broken", calls: &calls}}
	generic := GenericProvider{Run: func(string, ...string) ([]byte, error) { calls = append(calls, "app"); return nil, nil }}
	err := (Manager{Providers: []SurfaceProvider{broken, generic}}).ReturnToContext(ReturnTarget{AppBundleID: "app", Capability: ReturnApp})
	if err != nil || !reflect.DeepEqual(calls, []string{"app"}) {
		t.Fatal(err, calls)
	}
}

func TestV02NotificationWithoutLayers(t *testing.T) {
	for _, live := range []bool{true, false} {
		var target ReturnTarget
		// Shipped v0.2 field names, no Metadata or Layers.
		if err := json.Unmarshal([]byte(`{"Surface":"tabby","AppName":"Tabby","AppBundleID":"org.tabby","ContextID":"old-instance:old-context","WindowID":"","AgentSessionID":"old-agent","CWD":"/tmp","Capability":"exact_context"}`), &target); err != nil {
			t.Fatal(err)
		}
		if target.Layers != nil {
			t.Fatal("legacy payload invented layers")
		}
		var calls []string
		outer := stackProvider{name: "tabby", calls: &calls, valid: live}
		generic := GenericProvider{Run: func(string, ...string) ([]byte, error) { calls = append(calls, "app fallback"); return nil, nil }}
		if err := (Manager{Providers: []SurfaceProvider{outer, generic}}).ReturnToContext(target); err != nil {
			t.Fatal(err)
		}
		want := []string{"tabby probe", "tabby focus"}
		if !live {
			want = []string{"tabby probe", "app fallback"}
		}
		if !reflect.DeepEqual(calls, want) {
			t.Fatal(live, calls, want)
		}
	}
}
