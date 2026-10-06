package attention

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Resume is intentionally distinct from returning to an existing surface.
// No verified automatic provider ships in v0.5; terminal launch is not success.
type ResumeTarget struct {
	NativeSessionID string   `json:"native_session_id"`
	AgentFlavor     string   `json:"agent_flavor"`
	CWD             string   `json:"cwd"`
	Executable      string   `json:"executable"`
	Argv            []string `json:"argv"`
}
type ResumeResult struct {
	Target  *ResumeTarget `json:"target,omitempty"`
	Command string        `json:"command,omitempty"`
	Reason  string        `json:"reason,omitempty"`
	Pending bool          `json:"pending,omitempty"`
}
type ResumeProvider interface {
	Resume(ResumeTarget) (ResumeResult, error)
}

var nativeIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validNativeID(id string) bool { return nativeIDPattern.MatchString(id) }
func ShellQuote(s string) string   { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func ResumeArguments(s Session, executable string) (ResumeTarget, error) {
	t := ResumeTarget{NativeSessionID: s.NativeSessionID, AgentFlavor: s.AgentFlavor, CWD: s.CWD, Executable: executable}
	if s.RuntimeState != RuntimeExited {
		return t, errors.New("runtime has not reliably exited")
	}
	if !validNativeID(s.NativeSessionID) {
		return t, errors.New("reliable native session UUID unavailable")
	}
	if !filepath.IsAbs(s.CWD) {
		return t, errors.New("project directory must be absolute")
	}
	info, err := os.Stat(s.CWD)
	if err != nil || !info.IsDir() {
		return t, errors.New("project directory unavailable; select its new location before recovery")
	}
	if !filepath.IsAbs(executable) {
		return t, errors.New("trusted local CLI unavailable")
	}
	info, err = os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 {
		return t, errors.New("local CLI unavailable or unsafe")
	}
	switch s.AgentFlavor {
	case "claude_cli":
		t.Argv = []string{"--resume", s.NativeSessionID}
	case "codex_cli":
		t.Argv = []string{"resume", s.NativeSessionID}
	default:
		return t, errors.New("this agent source has no verified CLI recovery")
	}
	return t, nil
}

// Only search conventional, user-owned install locations, never hook payloads,
// CWD or ambient PATH. CLI --help verifies the installed command contract.
func Recovery(s Session) ResumeResult {
	if s.ConcurrentRuntime {
		return ResumeResult{Reason: "another runtime for this conversation may still be running"}
	}
	if s.RuntimeState != RuntimeExited {
		return ResumeResult{Reason: "runtime has not reliably exited"}
	}
	if s.ProcessIdentity != "" && ProcessProbe(s) != Exited {
		return ResumeResult{Reason: "original process still exists or cannot be verified; copy after it exits"}
	}
	name := ""
	switch s.AgentFlavor {
	case "claude_cli":
		name = "claude"
	case "codex_cli":
		name = "codex"
	default:
		return ResumeResult{Reason: "no verified recovery for this agent source"}
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".asdf", "shims"), "/opt/homebrew/bin", "/usr/local/bin"} {
		path := filepath.Join(dir, name)
		t, err := ResumeArguments(s, path)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		args := []string{"--help"}
		if name == "codex" {
			args = []string{"resume", "--help"}
		}
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Dir = home
		b, err := cmd.Output()
		cancel()
		if err != nil || (name == "claude" && !strings.Contains(string(b), "--resume")) || (name == "codex" && !strings.Contains(string(b), "SESSION_ID")) {
			continue
		}
		parts := []string{ShellQuote(path)}
		for _, arg := range t.Argv {
			parts = append(parts, ShellQuote(arg))
		}
		return ResumeResult{Target: &t, Command: "cd -- " + ShellQuote(t.CWD) + " && " + strings.Join(parts, " ")}
	}
	return ResumeResult{Reason: "native ID, project directory or compatible trusted CLI unavailable"}
}

// Menu labels use cached lifecycle evidence; clicks revalidate all conditions.
func SessionAction(s Session) (string, string) {
	if s.RuntimeState == RuntimeExited && s.RecoveryCapability == "supported" && !s.ConcurrentRuntime {
		name := ""
		switch s.AgentFlavor {
		case "claude_cli":
			name = "claude"
		case "codex_cli":
			name = "codex"
		}
		home, _ := os.UserHomeDir()
		if name != "" {
			for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".asdf", "shims"), "/opt/homebrew/bin", "/usr/local/bin"} {
				if _, err := ResumeArguments(s, filepath.Join(dir, name)); err == nil {
					return "copy_resume_command", ""
				}
			}
		}
	}
	if s.AgentFlavor == "codex_desktop" && s.ReturnTarget != nil && s.ReturnTarget.AppBundleID != "" {
		return "open_app", ""
	}
	if s.RuntimeState != RuntimeExited && s.SurfaceState != "unavailable" && s.ReturnTarget != nil {
		switch s.ReturnTarget.Capability {
		case "exact_context", "window":
			return "return", ""
		case "app":
			return "open_app", ""
		}
	}
	if s.ReturnTarget != nil && s.ReturnTarget.AppBundleID != "" {
		return "open_app", ""
	}
	if filepath.IsAbs(s.CWD) {
		if info, err := os.Stat(s.CWD); err == nil && info.IsDir() {
			return "open_project", ""
		}
	}
	return "", "No verified context, recovery or project is available"
}
