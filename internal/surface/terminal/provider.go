// Package terminal returns to an existing macOS Terminal.app tab via its public
// scripting dictionary. Detection never sends Apple events or requests access.
package terminal

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/surface"
)

//go:embed focus.applescript
var focusScript string
var ttyName = regexp.MustCompile(`^ttys[0-9]+$`)

type Context struct {
	TTY     string
	PID     int
	Started string
}
type Provider struct{ Run surface.Runner }

func (p Provider) runner() surface.Runner {
	if p.Run != nil {
		return p.Run
	}
	return func(name string, args ...string) ([]byte, error) {
		timeout := 1500 * time.Millisecond
		if name == "/usr/bin/osascript" {
			timeout = 45 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		if name == "/bin/ps" {
			// lstart is localized. The interactive shell and notification process
			// can have different locales, but must record the same process identity.
			cmd.Env = append(os.Environ(), "LC_ALL=C")
		}
		return cmd.CombinedOutput()
	}
}
func (p Provider) Name() string { return "terminal" }
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "terminal" && t.AppBundleID == "com.apple.Terminal" && t.Capability == surface.ReturnExactContext
}
func (p Provider) process(pid int) (parent int, tty, started string, err error) {
	output, err := p.runner()("/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "tty=", "-o", "lstart=")
	if err != nil {
		return 0, "", "", err
	}
	fields := strings.Fields(string(output))
	if len(fields) != 7 {
		return 0, "", "", errors.New("process identity unavailable")
	}
	parent, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", "", err
	}
	return parent, fields[1], strings.Join(fields[2:], " "), nil
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if explicit := c.Env("AGENTBELL_SURFACE"); explicit != "" && explicit != "terminal" {
		return nil, nil
	}
	target, err := (surface.GenericProvider{Run: p.Run}).Detect(c)
	if err != nil {
		return nil, err
	}
	// Apple_Terminal is Terminal's native marker; ancestry works without it.
	if c.Env("TERM_PROGRAM") == "Apple_Terminal" {
		target.Surface = "terminal"
		target.AppName = "Terminal"
		target.AppBundleID = "com.apple.Terminal"
		target.Capability = surface.ReturnApp
	}
	if target.AppBundleID != "com.apple.Terminal" {
		return nil, nil
	}
	target.Surface = "terminal"
	if c.Env("TMUX") != "" {
		return target, nil
	}
	pid := c.PID
	if pid == 0 {
		pid = os.Getpid()
	}
	var identity Context
	for i := 0; i < 32 && pid > 1; i++ {
		parent, tty, started, err := p.process(pid)
		if err != nil {
			break
		}
		if ttyName.MatchString(tty) {
			// Keep the highest ancestor on the same TTY (the login/shell session),
			// rather than the short-lived hook/agent. A different TTY is ambiguous.
			if identity.TTY != "" && identity.TTY != "/dev/"+tty {
				return target, nil
			}
			identity = Context{TTY: "/dev/" + tty, PID: pid, Started: started}
		}
		if parent == pid {
			break
		}
		pid = parent
	}
	if identity.TTY != "" {
		encoded, _ := json.Marshal(identity)
		target.ContextID = string(encoded)
		target.Capability = surface.ReturnExactContext
	}
	return target, nil
}
func parseContext(value string) (Context, error) {
	var identity Context
	if len(value) > 1024 {
		return identity, errors.New("Terminal context too large")
	}
	if err := json.Unmarshal([]byte(value), &identity); err != nil {
		return identity, err
	}
	if !strings.HasPrefix(identity.TTY, "/dev/") || !ttyName.MatchString(strings.TrimPrefix(identity.TTY, "/dev/")) || identity.PID <= 1 || len(strings.Fields(identity.Started)) != 5 {
		return identity, errors.New("invalid Terminal context")
	}
	return identity, nil
}
func (p Provider) Return(t surface.ReturnTarget) error {
	if !p.CanHandle(t) {
		return errors.New("unsupported Terminal target")
	}
	identity, err := parseContext(t.ContextID)
	if err != nil {
		return err
	}
	_, tty, started, err := p.process(identity.PID)
	if err != nil || "/dev/"+tty != identity.TTY || started != identity.Started {
		return errors.New("Terminal session expired")
	}
	// Context data is an argv value; it is never interpolated into script source.
	output, err := p.runner()("/usr/bin/osascript", "-e", focusScript, "--", identity.TTY)
	if err != nil {
		return fmt.Errorf("focus Terminal tab: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
