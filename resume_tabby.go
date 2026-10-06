package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/tabby"
)

// Native Tabby "run" uses structured argv and asks its own Run confirmation.
// The fixed AgentBell entry point supplies CWD without shell command strings.
type TabbyResumeProvider struct {
	Home    string
	Session attention.Session
	Start   func(string, []string) error
	Read    func() (*attention.Snapshot, error)
	Probe   func(surface.ReturnTarget) error
	Alive   func(attention.Session) bool
	Timeout time.Duration
}

func (p TabbyResumeProvider) Resume(target attention.ResumeTarget) (attention.ResumeResult, error) {
	result := attention.ResumeResult{Target: &target}
	expected, err := attention.ResumeArguments(p.Session, target.Executable)
	if err != nil || !reflect.DeepEqual(expected, target) || p.Session.ConcurrentRuntime || p.Session.DismissedAt != nil {
		return result, errors.New("recovery target changed or runtime is not eligible")
	}
	tabbyPath := attention.TabbyExecutable(p.Home)
	if tabbyPath == "" {
		return result, errors.New("Tabby is unavailable; use Copy Resume Command")
	}
	helper := filepath.Join(p.Home, "Applications", "AgentBell.app", "Contents", "MacOS", "agentbell")
	if info, err := os.Stat(helper); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 {
		return result, errors.New("installed AgentBell recovery launcher unavailable")
	}
	dir := filepath.Join(attention.Directory(p.Home), "recovery")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return result, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return result, errors.New("unsafe recovery lock directory")
	}
	lock, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(target.AgentFlavor+":"+target.NativeSessionID)))), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return result, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, errors.New("recovery for this conversation is already pending")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	start := p.Start
	if start == nil {
		start = func(path string, args []string) error {
			// A newly started Tabby GUI stays alive. Never time out and kill it.
			cmd := exec.Command(path, args...)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("Tabby launch failed: %w", err)
			}
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}
	requestedAt := time.Now()
	if err := start(tabbyPath, []string{"run", helper, "resume-launch", p.Session.ID}); err != nil {
		return result, err
	}
	read := p.Read
	if read == nil {
		read = func() (*attention.Snapshot, error) {
			response, err := attention.RequestTo(attention.SocketPath(p.Home), attention.Request{Version: 1, Command: "status"}, time.Second)
			return response.State, err
		}
	}
	probe := p.Probe
	if probe == nil {
		probe = (tabby.Provider{Directory: filepath.Join(p.Home, ".cache", "agentbell", "tabby")}).Probe
	}
	alive := p.Alive
	if alive == nil {
		alive = func(s attention.Session) bool { return attention.ProcessProbe(s) == attention.Alive }
	}
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 50 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := read()
		if err == nil && state != nil {
			sessions := append(append(append([]attention.Session{}, state.Working...), state.NeedsYou...), state.Recent...)
			for _, s := range sessions {
				if s.NativeSessionID == target.NativeSessionID && s.AgentFlavor == target.AgentFlavor && s.CWD == target.CWD && s.RuntimeState == attention.RuntimeRunning && s.RuntimeStartedAt != nil && !s.RuntimeStartedAt.Before(requestedAt.Add(-time.Second)) && s.RuntimeInstanceID != p.Session.RuntimeInstanceID && s.ReturnTarget != nil && s.ReturnTarget.Surface == "tabby" && s.ReturnTarget.Capability == surface.ReturnExactContext && s.ReturnTarget.ContextID != "" {
					if err := probe(*s.ReturnTarget); err == nil && alive(s) {
						return result, nil
					}
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, errors.New("recovery not confirmed: accept Tabby's Run prompt; the original UUID must start in its new tab. No new runtime was verified")
}

func resumeLaunchCommand(args []string) error {
	if len(args) != 1 || len(args[0]) > 1024 {
		return errors.New("usage: agentbell resume-launch SESSION_ID")
	}
	home, _ := os.UserHomeDir()
	response, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "get_session", SessionID: args[0]}, time.Second)
	if err != nil || response.Session == nil {
		return errors.New("original session unavailable")
	}
	if response.Session.DismissedAt != nil {
		return errors.New("session was dismissed")
	}
	result := attention.Recovery(*response.Session)
	if result.Target == nil {
		return errors.New(result.Reason)
	}
	target := *result.Target
	if target.AgentFlavor == "claude_cli" {
		if _, ok := attention.VerifyClaudeRecoveryIdentity(home, *response.Session); !ok {
			return errors.New("original Claude history cannot be verified in its saved directory")
		}
	}
	if err := os.Chdir(target.CWD); err != nil {
		return err
	}
	return syscall.Exec(target.Executable, append([]string{target.Executable}, target.Argv...), os.Environ())
}
