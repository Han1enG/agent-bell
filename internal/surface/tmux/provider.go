package tmux

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/identity"
)

var paneID = regexp.MustCompile(`^%[0-9]+$`)
var windowID = regexp.MustCompile(`^@[0-9]+$`)
var sessionID = regexp.MustCompile(`^\$[0-9]+$`)
var ttyID = regexp.MustCompile(`^/dev/(ttys[0-9]+|pts/[0-9]+)$`)

type Context struct{ PaneID, WindowID, SessionID, ServerSocket, SocketIdentity, ServerPID, ServerStarted, ClientTTY, ClientPID, ClientCreated string }
type Provider struct {
	Run            surface.Runner
	SocketIdentity func(string) (string, error)
}

func (Provider) Name() string { return "tmux" }
func (Provider) Capabilities() []surface.ReturnCapability {
	return []surface.ReturnCapability{surface.ReturnExactContext}
}
func (p Provider) run(args ...string) ([]byte, error) {
	if p.Run != nil {
		return p.Run("tmux", args...)
	}
	// Click-time validation/focus tolerates process scheduling on Intel hosts.
	// Hook detection replaces Run with the shared 90ms DetectionRunner.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.WaitDelay = 10 * time.Millisecond
	return cmd.CombinedOutput()
}
func (p Provider) stamp(path string) (string, error) {
	if p.SocketIdentity != nil {
		return p.SocketIdentity(path)
	}
	return identity.Socket(path)
}
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "tmux" && t.Capability == surface.ReturnExactContext
}

const format = "#{pane_id}\t#{window_id}\t#{session_id}\t#{pid}\t#{start_time}"
const clientsFormat = "#{client_tty}\t#{client_pid}\t#{client_created}\t#{session_id}\t#{client_control_mode}"

func (p Provider) snapshot(c Context) (Context, error) {
	out, err := p.run("-S", c.ServerSocket, "list-panes", "-s", "-t", c.SessionID, "-F", format)
	if err != nil {
		return c, surface.Fail(surface.ContextNotFound, errors.New("tmux session unavailable"))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) == 5 && f[0] == c.PaneID && windowID.MatchString(f[1]) && sessionID.MatchString(f[2]) && number(f[3]) && number(f[4]) {
			c.PaneID, c.WindowID, c.SessionID, c.ServerPID, c.ServerStarted = f[0], f[1], f[2], f[3], f[4]
			return c, nil
		}
	}
	return c, surface.Fail(surface.PaneNotFound, errors.New("tmux pane expired"))
}

func number(s string) bool { n, e := strconv.ParseUint(s, 10, 64); return e == nil && n > 0 }
func (p Provider) clients(c Context) ([][3]string, error) {
	out, err := p.run("-S", c.ServerSocket, "list-clients", "-F", clientsFormat)
	if err != nil {
		return nil, surface.Fail(surface.MultiplexerUnreachable, err)
	}
	var found [][3]string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) == 5 && f[3] == c.SessionID && f[4] == "0" && ttyID.MatchString(f[0]) && number(f[1]) && number(f[2]) {
			found = append(found, [3]string{f[0], f[1], f[2]})
		}
	}
	return found, nil
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	p.Run = surface.DetectionRunner(c, p.Run)
	if c.Env == nil {
		c.Env = os.Getenv
	}
	env := c.Env("TMUX")
	pane := c.Env("TMUX_PANE")
	if env == "" || pane == "" {
		return nil, nil
	}
	// TMUX ends in ,server-pid,session-index; custom paths may contain commas.
	end := strings.LastIndex(env, ",")
	if end < 0 {
		return nil, surface.Fail(surface.InvalidTarget, errors.New("invalid TMUX"))
	}
	prev := strings.LastIndex(env[:end], ",")
	if prev < 0 || !paneID.MatchString(pane) {
		return nil, surface.Fail(surface.InvalidTarget, errors.New("invalid TMUX identity"))
	}
	id := Context{PaneID: pane, ServerSocket: env[:prev]}
	stamp, err := p.stamp(id.ServerSocket)
	if err != nil {
		return nil, err
	}
	id.SocketIdentity = stamp
	// Use pane directly on detection; record the returned session ID for later probes.
	out, err := p.run("-S", id.ServerSocket, "display-message", "-p", "-t", pane, format)
	if err != nil {
		return nil, surface.Fail(surface.MultiplexerUnreachable, err)
	}
	f := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(f) != 5 || f[0] != pane || !windowID.MatchString(f[1]) || !sessionID.MatchString(f[2]) || f[3] != env[prev+1:end] || !number(f[3]) || !number(f[4]) {
		return nil, surface.Fail(surface.InvalidTarget, errors.New("tmux server identity unavailable"))
	}
	id.WindowID, id.SessionID, id.ServerPID, id.ServerStarted = f[1], f[2], f[3], f[4]
	clients, err := p.clients(id)
	if err != nil {
		return nil, err
	}
	capability := surface.ReturnApp
	if len(clients) == 1 {
		id.ClientTTY, id.ClientPID, id.ClientCreated = clients[0][0], clients[0][1], clients[0][2]
		capability = surface.ReturnExactContext
	}
	encoded, _ := json.Marshal(id)
	return &surface.ReturnTarget{Surface: "tmux", ContextID: string(encoded), WindowID: id.WindowID, CWD: c.CWD, Capability: capability, Metadata: map[string]string{"client_pid": id.ClientPID, "client_tty": id.ClientTTY, "tmux_pane": id.PaneID, "tmux_session": id.SessionID, "tmux_window": id.WindowID}}, nil
}
func parse(t surface.ReturnTarget) (Context, error) {
	var c Context
	if len(t.ContextID) > 4096 {
		return c, surface.Fail(surface.InvalidTarget, errors.New("tmux context too large"))
	}
	err := json.Unmarshal([]byte(t.ContextID), &c)
	if err != nil || !paneID.MatchString(c.PaneID) || !windowID.MatchString(c.WindowID) || !sessionID.MatchString(c.SessionID) || !number(c.ServerPID) || !number(c.ServerStarted) || c.SocketIdentity == "" || !ttyID.MatchString(c.ClientTTY) || !number(c.ClientPID) || !number(c.ClientCreated) {
		return c, surface.Fail(surface.InvalidTarget, errors.New("invalid tmux context"))
	}
	return c, nil
}
func (p Provider) Probe(t surface.ReturnTarget) error {
	if !p.CanHandle(t) {
		return surface.Fail(surface.UnsupportedSurface, errors.New("unsupported tmux target"))
	}
	c, err := parse(t)
	if err != nil {
		return err
	}
	stamp, err := p.stamp(c.ServerSocket)
	if err != nil {
		return err
	}
	if stamp != c.SocketIdentity {
		return surface.Fail(surface.ServerIdentityMismatch, errors.New("tmux socket generation changed"))
	}
	live, err := p.snapshot(c)
	if err != nil {
		return err
	}
	if live.ServerPID != c.ServerPID || live.ServerStarted != c.ServerStarted {
		return surface.Fail(surface.ServerIdentityMismatch, errors.New("tmux server restarted"))
	}
	if live.PaneID != c.PaneID || live.WindowID != c.WindowID || live.SessionID != c.SessionID {
		return surface.Fail(surface.ContextNotFound, errors.New("tmux pane moved or expired"))
	}
	clients, err := p.clients(c)
	if err != nil {
		return err
	}
	if len(clients) != 1 || clients[0] != [3]string{c.ClientTTY, c.ClientPID, c.ClientCreated} {
		return surface.Fail(surface.ContextNotFound, errors.New("original tmux client detached or ambiguous"))
	}
	return nil
}
func (p Provider) Return(t surface.ReturnTarget) error {
	if err := p.Probe(t); err != nil {
		return err
	}
	c, _ := parse(t)
	// Stable IDs only. No switch-client is needed: Probe requires the original
	// client still attached to this session. -Z preserves the zoom/layout state.
	if _, err := p.run("-S", c.ServerSocket, "select-window", "-t", c.SessionID+":"+c.WindowID); err != nil {
		return surface.Fail(surface.ContextNotFound, err)
	}
	if err := p.Probe(t); err != nil {
		return err
	}
	if _, err := p.run("-S", c.ServerSocket, "select-pane", "-Z", "-t", c.PaneID); err != nil {
		return surface.Fail(surface.ContextNotFound, err)
	}
	out, err := p.run("-S", c.ServerSocket, "display-message", "-p", "-c", c.ClientTTY, "-t", c.SessionID+":", "#{pane_id}")
	if err != nil || strings.TrimSpace(string(out)) != c.PaneID {
		return surface.Fail(surface.ContextNotFound, errors.New("tmux final selection unconfirmed"))
	}
	return nil
}

// ClientTTY decodes provider-owned identity. Core never trusts a free-form
// metadata field as authority to select an inner context.
func (p Provider) ClientTTY(t surface.ReturnTarget) (string, error) {
	c, err := parse(t)
	return c.ClientTTY, err
}

func (p Provider) ClientPID(t surface.ReturnTarget) (int, error) {
	c, err := parse(t)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(c.ClientPID)
}
