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
func ProcessAlive(s Session) bool {
	if s.ProcessID <= 1 || s.ProcessIdentity == "" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(s.ProcessID), "-o", "lstart=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	b, err := cmd.Output()
	if err != nil {
		// A successful ps with no matching PID exits 1; timeouts and permission
		// failures are inconclusive and must not discard live attention.
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return false
		}
		return true
	}
	return strings.Join(strings.Fields(string(b)), " ") == s.ProcessIdentity
}
