package attention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyRecoveryRequiresMatchingNativeHistoryMetadata(t *testing.T) {
	home := t.TempDir()
	s := Session{Agent: "claude", ID: "claude:12345678-1234-1234-1234-123456789abc", CWD: "/tmp/project", AgentFlavor: "unknown"}
	if _, ok := VerifyClaudeRecoveryIdentity(home, s); ok {
		t.Fatal("internal key was treated as native evidence")
	}
	dir := filepath.Join(home, ".claude", "projects", "-tmp-project")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "12345678-1234-1234-1234-123456789abc.jsonl")
	for _, invalid := range []string{`{"sessionId":"different","cwd":"/tmp/project"}`, `{"sessionId":"12345678-1234-1234-1234-123456789abc","cwd":"/other"}`, `invalid`} {
		os.WriteFile(path, []byte(invalid), 0600)
		if _, ok := VerifyClaudeRecoveryIdentity(home, s); ok {
			t.Fatal("unrelated or malformed history accepted")
		}
	}
	os.WriteFile(path, []byte(`{"sessionId":"12345678-1234-1234-1234-123456789abc","cwd":"/tmp/project","message":{"content":"private"}}`), 0600)
	verified, ok := VerifyClaudeRecoveryIdentity(home, s)
	if !ok || verified.NativeSessionID != "12345678-1234-1234-1234-123456789abc" || verified.AgentFlavor != "claude_cli" || verified.Summary != "" {
		t.Fatal("matching metadata not adopted without chat content", verified)
	}
	s.AgentFlavor = "codex_desktop"
	if _, ok := VerifyClaudeRecoveryIdentity(home, s); ok {
		t.Fatal("desktop source was converted to CLI")
	}
}
