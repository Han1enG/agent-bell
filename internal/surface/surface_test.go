package surface

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDetectionUnknownAndOrigin(t *testing.T) {
	for _, origin := range []bool{false, true} {
		p := GenericProvider{Run: func(name string, args ...string) ([]byte, error) {
			if strings.HasSuffix(name, "PlistBuddy") {
				return []byte("org.example.Unknown\n"), nil
			}
			if origin {
				return []byte("1 /Applications/Unknown App.app/Contents/MacOS/Unknown App\n"), nil
			}
			return []byte("1 /bin/zsh\n"), nil
		}}
		target, err := p.Detect(DetectContext{PID: 77, CWD: "/tmp", Env: func(string) string { return "" }})
		if err != nil {
			t.Fatal(err)
		}
		want := ReturnProject
		if origin {
			want = ReturnApp
		}
		if target.Capability != want || target.CWD != "/tmp" {
			t.Fatalf("target=%+v", target)
		}
		if origin && target.AppBundleID != "org.example.Unknown" {
			t.Fatal(target)
		}
	}
}
func TestProtocolDoesNotInventExactCapability(t *testing.T) {
	p := GenericProvider{Run: func(string, ...string) ([]byte, error) { return nil, errors.New("no processes") }}
	target, _ := p.Detect(DetectContext{Env: func(key string) string {
		if key == "AGENTBELL_SURFACE" {
			return "future"
		}
		if key == "AGENTBELL_CONTEXT_ID" {
			return "opaque"
		}
		return ""
	}})
	if target.Surface != "future" || target.ContextID != "opaque" || target.Capability != ReturnProject {
		t.Fatal(target)
	}
}

type failingProvider struct {
	attempts *[]ReturnCapability
	success  ReturnCapability
}

func (p failingProvider) Name() string                                { return "fixture" }
func (p failingProvider) Detect(DetectContext) (*ReturnTarget, error) { return nil, nil }
func (p failingProvider) CanHandle(ReturnTarget) bool                 { return true }
func (p failingProvider) Return(t ReturnTarget) error {
	*p.attempts = append(*p.attempts, t.Capability)
	if t.Capability == p.success {
		return nil
	}
	return errors.New("expired")
}
func TestFallbackChainExpiredContext(t *testing.T) {
	var attempts []ReturnCapability
	var observed []ReturnCapability
	target := ReturnTarget{ContextID: "expired", WindowID: "closed", AppBundleID: "org.example", CWD: "/tmp", Capability: ReturnExactContext}
	err := (Manager{
		Providers: []SurfaceProvider{failingProvider{&attempts, ReturnProject}},
		OnAttempt: func(provider string, capability ReturnCapability, err error) {
			observed = append(observed, capability)
			if provider != "fixture" || (err == nil) != (capability == ReturnProject) {
				t.Fatal("incorrect attempt result", provider, capability, err)
			}
		},
	}).ReturnToContext(target)
	if err != nil || !reflect.DeepEqual(attempts, []ReturnCapability{ReturnExactContext, ReturnWindow, ReturnApp, ReturnProject}) {
		t.Fatalf("%v %v", attempts, err)
	}
	if !reflect.DeepEqual(observed, attempts) {
		t.Fatal("fallback hid failed attempts", observed, attempts)
	}
}
func TestActivationAndProjectValidation(t *testing.T) {
	var args []string
	p := GenericProvider{FallbackApp: "Tabby", Run: func(_ string, a ...string) ([]byte, error) { args = a; return nil, nil }}
	if err := p.Return(ReturnTarget{Capability: ReturnApp, AppBundleID: `org.example;$(touch bad)`}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"-b", `org.example;$(touch bad)`}) {
		t.Fatal(args)
	}
	cwd := t.TempDir()
	if err := p.Return(ReturnTarget{Capability: ReturnProject, CWD: cwd}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"-a", "Tabby", "--", cwd}) {
		t.Fatal(args)
	}
	file := cwd + "/file"
	_ = os.WriteFile(file, []byte("x"), 0600)
	for _, bad := range []string{"", "relative", file, cwd + "/gone"} {
		if p.Return(ReturnTarget{Capability: ReturnProject, CWD: bad}) == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestActionTitles(t *testing.T) {
	for _, tc := range []struct {
		cap        ReturnCapability
		name, want string
	}{{ReturnExactContext, "", "返回会话"}, {ReturnWindow, "", "返回窗口"}, {ReturnApp, "Tabby", "打开 Tabby"}, {ReturnApp, "", "打开应用"}, {ReturnProject, "", "打开项目"}, {"", "", ""}} {
		if got := ActionTitle(ReturnTarget{Capability: tc.cap, AppName: tc.name}); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
}
