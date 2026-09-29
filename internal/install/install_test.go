package install

import (
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
	data, _ = os.ReadFile(claudePath)
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatal(err)
	}
	if countAgentBell(claude) != 0 {
		t.Fatal("AgentBell hooks were not fully removed")
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
