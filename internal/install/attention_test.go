package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttentionBundleUpgradeLoginAndConfigPreservation(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "AgentBell.app")
	mac := filepath.Join(source, "Contents", "MacOS")
	if err := os.MkdirAll(mac, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AgentBellApp", "AgentBellNotifier", "agentbell"} {
		if err := os.WriteFile(filepath.Join(mac, name), []byte("first"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "Contents", "Info.plist"), []byte("com.agentbell.AgentBell"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", filepath.Join(mac, "AgentBellNotifier"))
	cfg := filepath.Join(home, ".config", "agentbell", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("user config"), 0600); err != nil {
		t.Fatal(err)
	}
	installer := New(home, filepath.Join(mac, "agentbell"))
	app, err := installer.InstallAttentionApp(true)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(AttentionLoginPath(home)); err != nil || !strings.Contains(string(b), app) {
		t.Fatal("login launch missing", err)
	}
	if err = os.WriteFile(filepath.Join(mac, "AgentBellApp"), []byte("second"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = installer.InstallAttentionApp(false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(app, "Contents", "MacOS", "AgentBellApp")); string(b) != "second" {
		t.Fatal("bundle upgrade failed")
	}
	if _, err = os.Stat(AttentionLoginPath(home)); !os.IsNotExist(err) {
		t.Fatal("disabled login launch retained")
	}
	if b, _ := os.ReadFile(cfg); string(b) != "user config" {
		t.Fatal("config overwritten")
	}
}
