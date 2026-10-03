package iterm

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/surface"
)

//go:embed focus.applescript
var script string
var uuid = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var ttyRE = regexp.MustCompile(`^/dev/ttys[0-9]+$`)

type Context struct {
	SessionID       string
	AppPID          int
	AppStarted, TTY string
}
type Provider struct{ Run surface.Runner }

func (Provider) Name() string { return "iterm" }
func (Provider) Capabilities() []surface.ReturnCapability {
	return []surface.ReturnCapability{surface.ReturnApp, surface.ReturnExactContext}
}
func (p Provider) run(name string, args ...string) ([]byte, error) {
	if p.Run != nil {
		return p.Run(name, args...)
	}
	budget := 75 * time.Millisecond
	if name == "/usr/bin/osascript" {
		budget = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd.CombinedOutput()
}
func (p Provider) process(pid int) (int, string, string, string, error) {
	out, err := p.run("/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "tty=", "-o", "lstart=", "-o", "comm=")
	f := strings.Fields(string(out))
	if err != nil || len(f) < 8 {
		return 0, "", "", "", errors.New("process unavailable")
	}
	parent, e := strconv.Atoi(f[0])
	return parent, "/dev/" + f[1], strings.Join(f[2:7], " "), strings.Join(f[7:], " "), e
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	p.Run = surface.DetectionRunner(c, p.Run)
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.Env("TERM_PROGRAM") != "iTerm.app" && c.Env("ITERM_SESSION_ID") == "" && c.Env("AGENTBELL_SURFACE") != "iterm" {
		return nil, nil
	}
	if explicit := c.Env("AGENTBELL_SURFACE"); explicit != "" && explicit != "iterm" {
		return nil, nil
	}
	t := &surface.ReturnTarget{Surface: "iterm", AppName: "iTerm2", AppBundleID: "com.googlecode.iterm2", CWD: c.CWD, AgentSessionID: c.AgentSessionID, Capability: surface.ReturnApp}
	session := c.Env("ITERM_SESSION_ID")
	if index := strings.LastIndex(session, ":"); index >= 0 {
		session = session[index+1:]
	}
	if !uuid.MatchString(session) {
		return t, nil
	}
	id := Context{SessionID: session}
	pid := c.PID
	if pid == 0 {
		pid = os.Getpid()
	}
	for i := 0; i < 32 && pid > 1; i++ {
		parent, tty, started, comm, err := p.process(pid)
		if err != nil {
			break
		}
		if id.TTY == "" && ttyRE.MatchString(tty) {
			id.TTY = tty
		}
		if strings.Contains(comm, "iTerm.app/Contents/") || strings.Contains(comm, "iTerm2.app/Contents/") {
			id.AppPID, id.AppStarted = pid, started
			break
		}
		if parent == pid {
			break
		}
		pid = parent
	}
	if id.AppPID > 1 && id.AppStarted != "" && ttyRE.MatchString(id.TTY) {
		encoded, _ := json.Marshal(id)
		t.ContextID = string(encoded)
		t.Capability = surface.ReturnExactContext
	}
	return t, nil
}
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "iterm" && t.Capability == surface.ReturnExactContext
}
func parse(t surface.ReturnTarget) (Context, error) {
	var c Context
	if len(t.ContextID) > 2048 {
		return c, surface.Fail(surface.InvalidTarget, errors.New("iTerm context too large"))
	}
	err := json.Unmarshal([]byte(t.ContextID), &c)
	if err != nil || !uuid.MatchString(c.SessionID) || c.AppPID <= 1 || len(strings.Fields(c.AppStarted)) != 5 || !ttyRE.MatchString(c.TTY) {
		return c, surface.Fail(surface.InvalidTarget, errors.New("invalid iTerm identity"))
	}
	return c, nil
}
func (p Provider) execute(t surface.ReturnTarget, mode string) error {
	if !p.CanHandle(t) {
		return surface.Fail(surface.UnsupportedSurface, errors.New("unsupported iTerm target"))
	}
	c, err := parse(t)
	if err != nil {
		return err
	}
	_, _, start, comm, err := p.process(c.AppPID)
	if err != nil || start != c.AppStarted || !(strings.Contains(comm, "iTerm.app/Contents/") || strings.Contains(comm, "iTerm2.app/Contents/")) {
		return surface.Fail(surface.ContextNotFound, errors.New("iTerm app generation expired"))
	}
	out, err := p.run("/usr/bin/osascript", "-e", script, "--", c.SessionID, c.TTY, mode)
	if err == nil {
		return nil
	}
	reason := surface.ProviderUnavailable
	switch {
	case strings.Contains(string(out), "-1743"):
		reason = surface.PermissionDenied
	case strings.Contains(string(out), "context_not_found"):
		reason = surface.ContextNotFound
	case strings.Contains(string(out), "app_not_running"):
		reason = surface.AppNotRunning
	}
	return surface.Fail(reason, errors.New("iTerm automation failed"))
}
func (p Provider) Probe(t surface.ReturnTarget) error { return p.execute(t, "probe") }
func (p Provider) Return(t surface.ReturnTarget) error {
	if err := p.Probe(t); err != nil {
		return err
	}
	return p.execute(t, "focus")
}
func (p Provider) VerifyClient(t surface.ReturnTarget, tty string) error {
	c, err := parse(t)
	if err != nil {
		return err
	}
	if c.TTY != tty {
		return surface.Fail(surface.InstanceMismatch, errors.New("iTerm client TTY mismatch"))
	}
	return nil
}

func (p Provider) VerifyClientDuringDetection(c surface.DetectContext, t surface.ReturnTarget, tty string) error {
	p.Run = surface.DetectionRunner(c, p.Run)
	return p.VerifyClient(t, tty)
}
