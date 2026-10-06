package attention

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A legacy key alone is not a recovery identity. Verify the UUID against the
// native CLI's saved history and its original directory before adopting it.
func VerifyClaudeRecoveryIdentity(home string, s Session) (Session, bool) {
	if s.Agent != "claude" || s.DismissedAt != nil || !filepath.IsAbs(s.CWD) {
		return s, false
	}
	id := s.NativeSessionID
	if id == "" && strings.HasPrefix(s.ID, "claude:") {
		id = strings.TrimPrefix(s.ID, "claude:")
	}
	if !validNativeID(id) || (s.AgentFlavor != "" && s.AgentFlavor != "unknown" && s.AgentFlavor != "claude_cli") {
		return s, false
	}
	folder := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(s.CWD, "-")
	path := filepath.Join(home, ".claude", "projects", folder, id+".jsonl")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return s, false
	}
	f, err := os.Open(path)
	if err != nil {
		return s, false
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, 4<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var metadata struct {
			SessionID string `json:"sessionId"`
			CWD       string `json:"cwd"`
		}
		if json.Unmarshal(scanner.Bytes(), &metadata) == nil && metadata.SessionID == id && metadata.CWD == s.CWD {
			s.NativeSessionID, s.AgentFlavor, s.RecoveryCapability = id, "claude_cli", "supported"
			return s, true
		}
	}
	return s, false
}

func TabbyExecutable(home string) string {
	for _, app := range []string{filepath.Join(home, "Applications", "Tabby.app"), "/Applications/Tabby.app"} {
		path := filepath.Join(app, "Contents", "MacOS", "Tabby")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 && info.Mode().Perm()&0022 == 0 {
			return path
		}
	}
	return ""
}
