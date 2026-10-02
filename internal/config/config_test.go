package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, err := Load(path)
	if err != nil || !c.Allows("done") || c.Terminal.App != "terminal" {
		t.Fatalf("unexpected defaults: %+v, %v", c, err)
	}
	if err := os.WriteFile(path, []byte("[notifications]\ndone = false\nneeds_input = true\nneeds_approval = true\nerror = true\n\n[terminal]\napp = \"iterm2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Allows("done") || !c.Allows("needs_input") || c.Terminal.App != "iterm2" {
		t.Fatalf("overrides not applied: %+v", c)
	}
}

func TestInvalidConfigFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[notifications]\ndone = abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid boolean")
	}
}

func TestCodexPermissionRequestsRequireOptIn(t *testing.T) {
	c := Defaults()
	if c.AllowsFor("codex", "needs_approval") {
		t.Fatal("default config must suppress pre-review Codex requests")
	}
	if !c.AllowsFor("codex", "done") || !c.AllowsFor("claude", "needs_approval") {
		t.Fatal("completion and Claude approvals must remain enabled")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[notifications]\ncodex_permission_requests = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || !c.AllowsFor("codex", "needs_approval") {
		t.Fatalf("explicit opt-in was ignored: %+v, %v", c, err)
	}
	c.Notifications.NeedsApproval = false
	if c.AllowsFor("codex", "needs_approval") || c.AllowsFor("claude", "needs_approval") {
		t.Fatal("global approval switch must still apply")
	}
}
