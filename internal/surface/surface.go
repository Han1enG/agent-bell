package surface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SurfaceType string
type ReturnCapability string

const (
	ReturnExactContext ReturnCapability = "exact_context"
	ReturnWindow       ReturnCapability = "window"
	ReturnApp          ReturnCapability = "app"
	ReturnProject      ReturnCapability = "project"
)

type ReturnTarget struct {
	Surface        string
	AppName        string
	AppBundleID    string
	ContextID      string
	WindowID       string
	AgentSessionID string
	CWD            string
	Capability     ReturnCapability
	Metadata       map[string]string `json:",omitempty"`
	Layers         []SurfaceLayer    `json:",omitempty"`
}
type DetectContext struct {
	Env                 func(string) string
	CWD, AgentSessionID string
	PID                 int
	Deadline            time.Time
	OnTiming            func(string, time.Duration)
}
type SurfaceProvider interface {
	Name() string
	Detect(DetectContext) (*ReturnTarget, error)
	CanHandle(ReturnTarget) bool
	Return(ReturnTarget) error
}
type Runner func(string, ...string) ([]byte, error)

func command(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Detection shares one best-effort deadline across all selected providers.
// Injected runners remain available to deterministic provider tests.
func DetectionRunner(c DetectContext, run Runner) Runner {
	if run != nil {
		return run
	}
	return func(name string, args ...string) ([]byte, error) {
		deadline := c.Deadline
		if deadline.IsZero() {
			deadline = time.Now().Add(90 * time.Millisecond)
		}
		if !time.Now().Before(deadline) {
			return nil, context.DeadlineExceeded
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.WaitDelay = 10 * time.Millisecond
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		return cmd.CombinedOutput()
	}
}

type GenericProvider struct {
	Run         Runner
	FallbackApp string
}

func (p GenericProvider) runner() Runner {
	if p.Run != nil {
		return p.Run
	}
	return command
}
func (p GenericProvider) Name() string { return "generic" }
func (p GenericProvider) CanHandle(t ReturnTarget) bool {
	return t.Capability == ReturnApp || t.Capability == ReturnProject
}
func (p GenericProvider) Detect(c DetectContext) (*ReturnTarget, error) {
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.PID == 0 {
		c.PID = os.Getpid()
	}
	run := DetectionRunner(c, p.Run)
	t := &ReturnTarget{Surface: "generic", CWD: c.CWD, AgentSessionID: c.AgentSessionID, Capability: ReturnProject}
	// The explicit protocol is opaque; capability depends on an available provider.
	t.Surface = c.Env("AGENTBELL_SURFACE")
	t.ContextID = c.Env("AGENTBELL_CONTEXT_ID")
	if t.Surface == "" {
		t.Surface = "generic"
	}
	// Tabby's TERM_PROGRAM is verified against its local session implementation.
	if t.Surface == "tabby" || c.Env("TERM_PROGRAM") == "Tabby" {
		for _, root := range []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")} {
			id, err := run("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleIdentifier", filepath.Join(root, "Tabby.app", "Contents", "Info.plist"))
			if err == nil && strings.TrimSpace(string(id)) != "" {
				t.AppName = "Tabby"
				t.AppBundleID = strings.TrimSpace(string(id))
				t.Capability = ReturnApp
				if t.Surface == "generic" {
					t.Surface = "tabby"
				}
				return t, nil
			}
		}
	}
	// Process ancestry finds the originating bundle, never the unrelated foreground app.
	pid := c.PID
	for i := 0; i < 32 && pid > 1; i++ {
		out, err := run("/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "comm=")
		if err != nil {
			break
		}
		line := strings.TrimSpace(string(out))
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		parent, _ := strconv.Atoi(fields[0])
		executable := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		if index := strings.Index(executable, ".app/Contents/"); index >= 0 {
			app := executable[:index+4]
			id, err := run("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleIdentifier", filepath.Join(app, "Contents", "Info.plist"))
			if err == nil && strings.TrimSpace(string(id)) != "" {
				t.AppBundleID = strings.TrimSpace(string(id))
				t.AppName = strings.TrimSuffix(filepath.Base(app), ".app")
				t.Capability = ReturnApp
				if t.Surface == "generic" {
					t.Surface = strings.ToLower(t.AppName)
				}
				break
			}
		}
		if parent == pid {
			break
		}
		pid = parent
	}
	return t, nil
}
func ActionTitle(t ReturnTarget) string {
	switch t.Capability {
	case ReturnExactContext:
		return "返回会话"
	case ReturnWindow:
		return "返回窗口"
	case ReturnApp:
		if t.AppName != "" {
			return "打开 " + t.AppName
		}
		return "打开应用"
	case ReturnProject:
		return "打开项目"
	}
	return ""
}
func (p GenericProvider) Return(t ReturnTarget) error {
	run := p.runner()
	if t.Capability == ReturnApp {
		if t.AppBundleID == "" {
			return Fail(InvalidTarget, errors.New("origin bundle unavailable"))
		}
		_, err := run("/usr/bin/open", "-b", t.AppBundleID)
		return err
	}
	if t.Capability != ReturnProject {
		return Fail(UnsupportedSurface, errors.New("unsupported capability"))
	}
	if !filepath.IsAbs(t.CWD) {
		return Fail(InvalidCWD, errors.New("project path must be absolute"))
	}
	info, err := os.Stat(t.CWD)
	if err != nil || !info.IsDir() {
		return Fail(InvalidCWD, errors.New("project directory unavailable"))
	}
	app := p.FallbackApp
	if app == "" || app == "auto" {
		// Prefer an installed terminal; Terminal is the final project fallback only.
		for _, candidate := range []string{"Tabby", "iTerm", "Ghostty", "WezTerm", "Warp", "Terminal"} {
			if _, err := os.Stat(filepath.Join("/Applications", candidate+".app")); err == nil {
				app = candidate
				break
			}
		}
		if app == "" || app == "auto" {
			app = "Terminal"
		}
	}
	switch strings.ToLower(app) {
	case "tabby":
		app = "Tabby"
	case "terminal":
		app = "Terminal"
	case "iterm2":
		app = "iTerm"
	}
	_, err = run("/usr/bin/open", "-a", app, "--", t.CWD)
	return err
}

type Manager struct {
	Providers []SurfaceProvider
	OnAttempt func(provider string, capability ReturnCapability, err error)
}

func (m Manager) ReturnToContext(t ReturnTarget) error {
	if len(t.Layers) > 0 {
		return m.returnComposite(t)
	}
	var failures []error
	for _, capability := range []ReturnCapability{ReturnExactContext, ReturnWindow, ReturnApp, ReturnProject} {
		if capability == ReturnExactContext && t.ContextID == "" || capability == ReturnWindow && t.WindowID == "" || capability == ReturnApp && t.AppBundleID == "" || capability == ReturnProject && t.CWD == "" {
			continue
		}
		attempt := t
		attempt.Capability = capability
		handled := false
		for _, p := range m.Providers {
			canHandle := false
			handleErr := Isolate(func() error { canHandle = p.CanHandle(attempt); return nil })
			if handleErr != nil {
				failures = append(failures, handleErr)
				continue
			}
			if canHandle {
				handled = true
				var err error
				if capability == ReturnExactContext {
					if probe, ok := p.(ProbeableProvider); ok {
						err = Isolate(func() error { return probe.Probe(attempt) })
					}
				}
				if err == nil {
					err = Isolate(func() error { return p.Return(attempt) })
				}
				if m.OnAttempt != nil {
					m.OnAttempt(p.Name(), capability, err)
				}
				if err == nil {
					return nil
				} else {
					failures = append(failures, fmt.Errorf("%s %s: %w", p.Name(), capability, err))
				}
			}
		}
		if !handled {
			err := Fail(ProviderUnavailable, errors.New("no provider for capability"))
			failures = append(failures, err)
			if m.OnAttempt != nil {
				m.OnAttempt("none", capability, err)
			}
		}
	}
	if len(failures) == 0 {
		return Fail(InvalidTarget, errors.New("no return destination available"))
	}
	return errors.Join(failures...)
}
func ReturnToContext(t ReturnTarget) error {
	return (Manager{Providers: []SurfaceProvider{GenericProvider{}}}).ReturnToContext(t)
}
func Encode(t *ReturnTarget) string { b, _ := json.Marshal(t); return string(b) }
