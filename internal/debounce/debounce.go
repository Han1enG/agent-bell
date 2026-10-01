package debounce

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/han1eng/agent-bell/internal/event"
)

const Window = 3 * time.Second

// Suppressed returns true when the same event key was recorded within Window.
// Cache failures intentionally fail open so they never block notifications.
func Suppressed(home string, e event.AgentEvent, now time.Time) bool {
	identity := e.SessionID
	if identity == "" {
		identity = e.CWD
	}
	if identity == "" {
		identity = e.Project
	}
	key := sha256.Sum256([]byte(e.Source + "\x00" + identity + "\x00" + string(e.Type)))
	dir := filepath.Join(home, "Library", "Caches", "AgentBell")
	if os.MkdirAll(dir, 0o700) != nil {
		return false
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".stamp" {
				continue
			}
			info, err := entry.Info()
			if err == nil && now.Sub(info.ModTime()) >= Window {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}
	path := filepath.Join(dir, hex.EncodeToString(key[:])+".stamp")
	if b, err := os.ReadFile(path); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, string(b)); err == nil && now.Sub(t) >= 0 && now.Sub(t) < Window {
			return true
		}
	}
	tmp, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return false
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0o600)
	if _, err = tmp.WriteString(now.Format(time.RFC3339Nano)); err != nil {
		tmp.Close()
		return false
	}
	if tmp.Close() != nil {
		return false
	}
	if err := os.Rename(name, path); err != nil {
		return false
	}
	return false
}
