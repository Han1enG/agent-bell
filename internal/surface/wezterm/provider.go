package wezterm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/notify"
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/identity"
)

type Context struct {
	PaneID                 uint64
	Socket, SocketIdentity string
	GUIPID                 int
	GUIStarted             string
}
type Pane struct {
	PaneID   uint64 `json:"pane_id"`
	TabID    uint64 `json:"tab_id"`
	WindowID uint64 `json:"window_id"`
}
type Provider struct {
	Run            surface.Runner
	SocketIdentity func(string) (string, error)
	NativeHelper   string
}

func (Provider) Name() string { return "wezterm" }
func (Provider) Capabilities() []surface.ReturnCapability {
	return []surface.ReturnCapability{surface.ReturnApp, surface.ReturnExactContext}
}
func (p Provider) stamp(path string) (string, error) {
	if p.SocketIdentity != nil {
		return p.SocketIdentity(path)
	}
	return identity.Socket(path)
}
func (p Provider) run(c Context, args ...string) ([]byte, error) {
	// Explicitly bind every invocation to the saved instance. Never GUI-discover.
	argv := append([]string{"WEZTERM_UNIX_SOCKET=" + c.Socket, "wezterm", "cli", "--no-auto-start"}, args...)
	if p.Run != nil {
		return p.Run("/usr/bin/env", argv...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "/usr/bin/env", argv...).CombinedOutput()
}
func (p Provider) system(name string, args ...string) ([]byte, error) {
	if p.Run != nil {
		return p.Run(name, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd.CombinedOutput()
}
func (p Provider) process(pid int) (string, error) {
	out, err := p.system("/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=", "-o", "comm=")
	f := strings.Fields(string(out))
	if err != nil || len(f) < 6 || !strings.Contains(strings.Join(f[5:], " "), "WezTerm.app/Contents/MacOS/wezterm-gui") {
		return "", surface.Fail(surface.AppNotRunning, errors.New("WezTerm GUI identity unavailable"))
	}
	return strings.Join(f[:5], " "), nil
}
func guiPID(socket string) int {
	base := filepath.Base(socket)
	if !strings.HasPrefix(base, "gui-sock-") {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimPrefix(base, "gui-sock-"))
	return pid
}
func (p Provider) helper() string {
	if p.NativeHelper != "" {
		return p.NativeHelper
	}
	executable, _ := os.Executable()
	return notify.NativeHelperFor(executable)
}
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "wezterm" && t.Capability == surface.ReturnExactContext
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	p.Run = surface.DetectionRunner(c, p.Run)
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.Env("TERM_PROGRAM") != "WezTerm" && c.Env("WEZTERM_PANE") == "" {
		return nil, nil
	}
	if c.Env("AGENTBELL_SURFACE") != "" && c.Env("AGENTBELL_SURFACE") != "wezterm" {
		return nil, nil
	}
	t := &surface.ReturnTarget{Surface: "wezterm", AppName: "WezTerm", AppBundleID: "org.wezfurlong.wezterm", CWD: c.CWD, AgentSessionID: c.AgentSessionID, Capability: surface.ReturnApp}
	pane, err := strconv.ParseUint(c.Env("WEZTERM_PANE"), 10, 64)
	if err != nil {
		return t, nil
	}
	socket := c.Env("WEZTERM_UNIX_SOCKET")
	stamp, err := p.stamp(socket)
	if err != nil {
		return t, nil
	}
	pid := guiPID(socket)
	if pid <= 1 {
		return t, nil
	}
	started, err := p.process(pid)
	if err != nil {
		return t, nil
	}
	encoded, _ := json.Marshal(Context{PaneID: pane, Socket: socket, SocketIdentity: stamp, GUIPID: pid, GUIStarted: started})
	t.ContextID = string(encoded)
	if p.helper() != "" {
		t.Capability = surface.ReturnExactContext
	}
	return t, nil
}
func parse(t surface.ReturnTarget) (Context, error) {
	var c Context
	if len(t.ContextID) > 4096 {
		return c, surface.Fail(surface.InvalidTarget, errors.New("WezTerm context too large"))
	}
	var raw struct {
		PaneID                 *uint64
		Socket, SocketIdentity string
		GUIPID                 int
		GUIStarted             string
	}
	if json.Unmarshal([]byte(t.ContextID), &raw) != nil || raw.PaneID == nil || raw.SocketIdentity == "" || raw.GUIPID <= 1 || guiPID(raw.Socket) != raw.GUIPID || len(strings.Fields(raw.GUIStarted)) != 5 {
		return c, surface.Fail(surface.InvalidTarget, errors.New("invalid WezTerm identity"))
	}
	return Context{PaneID: *raw.PaneID, Socket: raw.Socket, SocketIdentity: raw.SocketIdentity, GUIPID: raw.GUIPID, GUIStarted: raw.GUIStarted}, nil
}

type paneInfo struct {
	PaneID   *uint64 `json:"pane_id"`
	TabID    *uint64 `json:"tab_id"`
	WindowID *uint64 `json:"window_id"`
	TTY      string  `json:"tty_name"`
	Active   bool    `json:"is_active"`
}

func (p Provider) pane(t surface.ReturnTarget) (paneInfo, error) {
	if !p.CanHandle(t) {
		return paneInfo{}, surface.Fail(surface.UnsupportedSurface, errors.New("unsupported WezTerm target"))
	}
	c, err := parse(t)
	if err != nil {
		return paneInfo{}, err
	}
	stamp, err := p.stamp(c.Socket)
	if err != nil {
		return paneInfo{}, err
	}
	if stamp != c.SocketIdentity {
		return paneInfo{}, surface.Fail(surface.InstanceMismatch, errors.New("WezTerm mux generation changed"))
	}
	started, err := p.process(c.GUIPID)
	if err != nil || started != c.GUIStarted {
		return paneInfo{}, surface.Fail(surface.InstanceMismatch, errors.New("WezTerm GUI generation changed"))
	}
	out, err := p.run(c, "list", "--format", "json")
	if err != nil {
		return paneInfo{}, surface.Fail(surface.ProviderUnavailable, errors.New("WezTerm mux unreachable"))
	}
	var panes []paneInfo
	if len(out) > 1048576 || json.Unmarshal(out, &panes) != nil {
		return paneInfo{}, surface.Fail(surface.ProviderUnavailable, errors.New("invalid WezTerm list response"))
	}
	for _, pane := range panes {
		if pane.PaneID != nil && pane.TabID != nil && pane.WindowID != nil && *pane.PaneID == c.PaneID {
			return pane, nil
		}
	}
	return paneInfo{}, surface.Fail(surface.PaneNotFound, errors.New("WezTerm pane expired"))
}
func (p Provider) Probe(t surface.ReturnTarget) error {
	if p.helper() == "" {
		return surface.Fail(surface.ProviderUnavailable, errors.New("native helper required for exact GUI instance activation"))
	}
	_, err := p.pane(t)
	return err
}
func (p Provider) VerifyClient(t surface.ReturnTarget, tty string) error {
	pane, err := p.pane(t)
	if err != nil {
		return err
	}
	if tty == "" || pane.TTY != tty {
		return surface.Fail(surface.InstanceMismatch, errors.New("WezTerm pane TTY does not match tmux client"))
	}
	return nil
}

func (p Provider) Return(t surface.ReturnTarget) error {
	if err := p.Probe(t); err != nil {
		return err
	}
	c, _ := parse(t)
	helper := p.helper()
	if helper == "" {
		return surface.Fail(surface.ProviderUnavailable, errors.New("native helper required for exact GUI instance activation"))
	}
	if _, err := p.system(helper, "--activate-wezterm", strconv.Itoa(c.GUIPID)); err != nil {
		return surface.Fail(surface.AppNotRunning, errors.New("WezTerm GUI activation failed"))
	}
	if err := p.Probe(t); err != nil {
		return err
	}
	if _, err := p.run(c, "activate-pane", "--pane-id", strconv.FormatUint(c.PaneID, 10)); err != nil {
		return surface.Fail(surface.ContextNotFound, err)
	}
	pane, err := p.pane(t)
	if err != nil {
		return err
	}
	if !pane.Active {
		return surface.Fail(surface.ContextNotFound, errors.New("WezTerm final active pane unconfirmed"))
	}
	return nil
}

func (p Provider) VerifyClientDuringDetection(c surface.DetectContext, t surface.ReturnTarget, tty string) error {
	p.Run = surface.DetectionRunner(c, p.Run)
	return p.VerifyClient(t, tty)
}
