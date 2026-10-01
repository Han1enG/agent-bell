package install

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAndUninstallPreserveOtherSettings(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("installer is macOS-only")
	}
	home := t.TempDir()
	helper := filepath.Join(home, "AgentBellNotifier")
	if err := os.WriteFile(helper, []byte("test helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTBELL_NOTIFIER", helper)
	claudePath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, []byte(`{"model":"opus","hooks":{"Stop":[{"matcher":"*","hooks":[{"type":"command","command":"user-hook"}]}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	installer := New(home, "/tmp/agentbell")
	if err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	var claude map[string]any
	data, _ := os.ReadFile(claudePath)
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatal(err)
	}
	if claude["model"] != "opus" {
		t.Fatal("existing Claude settings were not preserved")
	}
	if countAgentBell(claude) != 4 {
		t.Fatalf("expected 4 Claude hooks, got %d", countAgentBell(claude))
	}
	if err := installer.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if err := installer.Uninstall(); err != nil {
		t.Fatalf("second uninstall should be safe: %v", err)
	}
	data, _ = os.ReadFile(claudePath)
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatal(err)
	}
	if countAgentBell(claude) != 0 {
		t.Fatal("AgentBell hooks were not fully removed")
	}
}

func TestPreviewDoesNotModifyAnyFiles(t *testing.T) {
	home := t.TempDir()
	claudePath := filepath.Join(home, ".claude", "settings.json")
	codexPath := filepath.Join(home, ".codex", "hooks.json")
	for _, path := range []string{claudePath, codexPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[]}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string][32]byte{}
	for _, path := range []string{claudePath, codexPath} {
		b, _ := os.ReadFile(path)
		before[path] = sha256.Sum256(b)
	}
	if _, err := Preview(home); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{claudePath, codexPath} {
		b, _ := os.ReadFile(path)
		if sha256.Sum256(b) != before[path] {
			t.Fatalf("preview changed %s", path)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agentbell")); !os.IsNotExist(err) {
		t.Fatal("preview created config directory")
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "Caches", "AgentBell")); !os.IsNotExist(err) {
		t.Fatal("preview created cache directory")
	}
}

func TestUninstallNeverRemovesHomebrewManagedApp(t *testing.T) {
	home := t.TempDir()
	app := filepath.Join(home, "Cellar", "agentbell", "0.2.0", "libexec", "AgentBell.app")
	executable := filepath.Join(app, "Contents", "MacOS", "agentbell")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("managed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := New(home, executable).Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("uninstall removed package-managed executable: %v", err)
	}
}

func countAgentBell(value any) int {
	count := 0
	switch value := value.(type) {
	case map[string]any:
		for _, child := range value {
			count += countAgentBell(child)
		}
	case []any:
		for _, child := range value {
			count += countAgentBell(child)
		}
	case string:
		if len(value) >= len(marker) && contains(value, marker) {
			count++
		}
	}
	return count
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
