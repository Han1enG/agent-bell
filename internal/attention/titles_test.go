package attention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeTitleMetadataAndSafeFallback(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-tmp-project")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions-index.json"), []byte(`{"entries":[{"sessionId":"other","customTitle":"Wrong session"},{"sessionId":"wanted","customTitle":"Fix login race","summary":"Generated summary"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LookupTitle(home, "claude", "wanted", "/tmp/project"); got != "Fix login race" {
		t.Fatal(got)
	}
	if got := LookupTitle(home, "claude", "missing", "/tmp/project"); got != "" {
		t.Fatal("used another session's title", got)
	}
	if CleanTitle("# Pasted prompt\ntext") != "" {
		t.Fatal("prompt-like multiline title accepted")
	}
}

func TestCleanTitleFiltersControls(t *testing.T) {
	if got := CleanTitle("Fix\x00 login\t bug"); got != "Fix login bug" {
		t.Fatalf("unexpected title %q", got)
	}
}
