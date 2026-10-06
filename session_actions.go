package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/builtin"
)

func sessionActionCommand(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: agentbell session-action SESSION_ID return|open_app|open_project|copy_resume_command")
	}
	home, _ := os.UserHomeDir()
	r, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "get_session", SessionID: args[0]}, time.Second)
	if err != nil {
		return err
	}
	if r.Session == nil {
		return errors.New("session unavailable")
	}
	s := *r.Session
	invalidate := func() {
		_, _ = attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "invalidate_context", SessionID: s.ID}, time.Second)
	}
	if s.DismissedAt != nil {
		return errors.New("session was dismissed")
	}
	switch args[1] {
	case "copy_resume_command":
		result := attention.Recovery(s)
		if result.Command == "" {
			return errors.New(result.Reason)
		}
		cmd := exec.Command("/usr/bin/pbcopy")
		input, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err = cmd.Start(); err != nil {
			return err
		}
		_, writeErr := input.Write([]byte(result.Command))
		input.Close()
		err = cmd.Wait()
		if writeErr != nil {
			return writeErr
		}
		return err
	case "open_project":
		if s.CWD == "" {
			return errors.New("project directory unavailable")
		}
		info, err := os.Stat(s.CWD)
		if err != nil || !info.IsDir() {
			return errors.New("project directory moved or deleted")
		}
		return exec.Command("/usr/bin/open", "--", s.CWD).Run()
	case "open_app":
		if s.ReturnTarget == nil || s.ReturnTarget.AppBundleID == "" {
			return errors.New("source application unavailable")
		}
		return exec.Command("/usr/bin/open", "-b", s.ReturnTarget.AppBundleID).Run()
	case "return":
		if s.RuntimeState == attention.RuntimeExited || attention.ProcessProbe(s) == attention.Exited {
			invalidate()
			return errors.New("runtime has exited; use recovery or Open Project")
		}
		if s.ReturnTarget == nil {
			return errors.New("original context unavailable")
		}
		registry := builtin.Registry("")
		if err := returnSessionContext(registry, *s.ReturnTarget); err != nil {
			invalidate()
			return err
		}
		return nil
	default:
		return errors.New("unsupported session action")
	}
}

// Exact menu Return never treats app activation or a different tab as success.
// The legacy notification manager retains its Universal Return fallback policy.
func returnSessionContext(registry surface.Registry, target surface.ReturnTarget) error {
	targets := []surface.ReturnTarget{target}
	if len(target.Layers) > 0 {
		if len(target.Layers) != 2 || target.Layers[1].Provider != "tmux" {
			return errors.New("unsupported original context stack")
		}
		targets = nil
		for _, layer := range target.Layers {
			targets = append(targets, layer.Target(target))
		}
	}
	for _, t := range targets {
		p := registry.Provider(t.Surface)
		if p == nil {
			return errors.New("original surface provider unavailable")
		}
		probe, ok := p.(surface.ProbeableProvider)
		if !ok {
			return errors.New("original context cannot be verified; use Open App")
		}
		if err := probe.Probe(t); err != nil {
			return fmt.Errorf("original context unavailable: %w", err)
		}
	}
	if len(targets) == 2 {
		outer, inner := registry.Provider(targets[0].Surface), registry.Provider(targets[1].Surface)
		binding, ok := outer.(surface.ClientBindingProvider)
		if !ok {
			return errors.New("original terminal attachment cannot be verified")
		}
		attached, ok := inner.(surface.AttachedClientProvider)
		if !ok {
			return errors.New("original tmux client cannot be verified")
		}
		tty, err := attached.ClientTTY(targets[1])
		if err != nil {
			return err
		}
		if err := binding.VerifyClient(targets[0], tty); err != nil {
			return fmt.Errorf("original terminal attachment unavailable: %w", err)
		}
	}
	for _, t := range targets {
		if err := registry.Provider(t.Surface).Return(t); err != nil {
			return fmt.Errorf("original context return failed: %w", err)
		}
	}
	return nil
}
