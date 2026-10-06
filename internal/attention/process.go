package attention

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// OriginProcess searches executable ancestry only; never reads arguments, env,
// terminal contents or prompts. Shells and transient hook processes are not agents.
func OriginProcess(source string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	pid := os.Getppid()
	for i := 0; i < 16 && pid > 1; i++ {
		cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "lstart=", "-o", "comm=")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		b, err := cmd.Output()
		if err != nil {
			return 0, ""
		}
		f := strings.Fields(string(b))
		if len(f) < 7 {
			return 0, ""
		}
		executable := strings.Join(f[6:], " ")
		name := strings.ToLower(filepath.Base(executable))
		if name == source && !strings.Contains(executable, ".app/Contents/MacOS/") {
			return pid, strings.Join(f[1:6], " ")
		}
		parent, _ := strconv.Atoi(f[0])
		if parent == pid {
			break
		}
		pid = parent
	}
	return 0, ""
}

// ProcessProbe never uses a missing surface or unreachable bridge as exit evidence.
func ProcessProbe(s Session) ProbeResult {
	if s.ProcessID <= 1 || s.ProcessIdentity == "" {
		return Unknown
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(s.ProcessID), "-o", "lstart=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	b, err := cmd.Output()
	if ctx.Err() != nil {
		return Unknown
	}
	return classifyProcessProbe(s.ProcessIdentity, b, err)
}
func classifyProcessProbe(identity string, b []byte, err error) ProbeResult {
	if err != nil {
		// ps exit 1 is authoritative only with empty output and no diagnostic.
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 && strings.TrimSpace(string(b)) == "" && len(e.Stderr) == 0 {
			return Exited
		}
		return Unknown
	}
	actual := strings.Join(strings.Fields(string(b)), " ")
	if actual == "" {
		return Unknown
	}
	if actual != identity {
		return Exited
	}
	return Alive
}
func ProcessAlive(s Session) bool { return ProcessProbe(s) != Exited }
