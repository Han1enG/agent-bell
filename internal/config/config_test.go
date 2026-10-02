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

func TestReturnConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if Defaults().Return.FallbackApp != "auto" || !Defaults().Return.Enabled {
		t.Fatal("invalid return defaults")
	}
	if err := os.WriteFile(path, []byte("[return]\nenabled = false\nfallback_app = \"tabby\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Return.Enabled || cfg.Return.FallbackApp != "tabby" {
		t.Fatal(cfg, err)
	}
}

func TestPermissionRequestRequiresSeparateOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, content := range []string{"", "[notifications]\nneeds_approval = true\n"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil || cfg.Allows("permission_request") || !cfg.Allows("needs_approval") {
			t.Fatalf("request enabled by legacy/default config: %+v %v", cfg, err)
		}
	}
	if err := os.WriteFile(path, []byte("[notifications]\npermission_request = true\nneeds_approval = false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || !cfg.Allows("permission_request") || cfg.Allows("needs_approval") {
		t.Fatalf("independent opt-in failed: %+v %v", cfg, err)
	}
}
