//go:build cgo

package attention

import (
	"github.com/han1eng/agent-bell/internal/event"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteMigrationRestorePrivacyAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentbell.db")
	s, err := OpenStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := s.scalar("PRAGMA user_version"); err != nil || version != "1" {
		t.Fatal("migration", version, err)
	}
	m := New(7)
	e := fixture("a", event.NeedsInput, time.Now())
	e.Raw = []byte(`{"prompt":"SECRET_PROMPT"}`)
	e.Message = "short summary"
	apply(t, m, e)
	m.Paused = true
	if err = s.Save(m, []Transition{{ID: "claude:a", Type: "needs_input", Timestamp: e.Timestamp.Format(time.RFC3339Nano)}}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(path, true)
	if err != nil {
		t.Fatal(err)
	}
	restored := New(7)
	if err = s.Load(restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Paused || len(restored.Snapshot().NeedsYou) != 1 || restored.Sessions["claude:a"].Summary != "short summary" {
		t.Fatal("restore mismatch")
	}
	if err = s.Health(); err != nil {
		t.Fatal(err)
	}
	s.Close()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if contains(bytes, []byte("SECRET_PROMPT")) {
		t.Fatal("raw hook stored")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("store not private")
	}
	s, err = OpenStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	apply(t, m, fixture("a", event.Done, e.Timestamp.Add(time.Second)))
	m.ClearRecent()
	if err = s.Save(m, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.scalar("SELECT count(*) FROM sessions"); n != "1" {
		t.Fatal("dismissal tombstone not persisted")
	}
	if n, _ := s.scalar("SELECT count(*) FROM events"); n != "1" {
		t.Fatal("dismissed transition lost")
	}
	if err = s.exec("PRAGMA user_version=2"); err != nil {
		t.Fatal(err)
	}
	if other, e := OpenStore(path, true); e == nil {
		other.Close()
		t.Fatal("future schema accepted")
	}
}
func contains(a, b []byte) bool {
	for i := 0; i+len(b) <= len(a); i++ {
		match := true
		for j := range b {
			if a[i+j] != b[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestCodexDisplayTitleMetadataOverridesPromptFallback(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sqlite", "codex-dev.db")
	s, err := OpenStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.exec("CREATE TABLE local_thread_catalog(thread_id TEXT,host_id TEXT,display_title TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err = s.put("INSERT INTO local_thread_catalog VALUES(?,?,?)", "wanted", "local", "Build AgentBell v0.4 Attention"); err != nil {
		t.Fatal(err)
	}
	if err = s.put("INSERT INTO local_thread_catalog VALUES(?,?,?)", "wanted", "remote", "Wrong host"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := LookupTitle(home, "codex", "wanted", ""); got != "Build AgentBell v0.4 Attention" {
		t.Fatal(got)
	}
	if got := LookupTitle(home, "codex", "missing", ""); got != "" {
		t.Fatal("title leaked across sessions", got)
	}
}

func TestRestoreLegacyWorkingAgeFromRealJournal(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "agentbell.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m := New(7)
	start := time.Now()
	next := start.Add(time.Hour)
	apply(t, m, fixture("a", event.Working, start))
	apply(t, m, fixture("a", event.Working, next))
	v := m.Sessions["claude:a"]
	v.WorkingAt = nil
	m.Sessions[v.ID] = v
	if err := s.Save(m, []Transition{{ID: v.ID, Type: "working", Timestamp: next.Format(time.RFC3339Nano)}}); err != nil {
		t.Fatal(err)
	}
	restored := New(7)
	if err := s.Load(restored); err != nil {
		t.Fatal(err)
	}
	got := restored.Sessions[v.ID].WorkingAt
	if got == nil || !got.Equal(next) {
		t.Fatal("lost recorded turn start", got)
	}
}

func TestDismissRestoresWatermarkWithoutAttention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	store, err := OpenStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	m := New(7)
	e := fixture("a", event.NeedsInput, now)
	apply(t, m, e)
	m.Dismiss("claude:a", now.Add(time.Second))
	if err := store.Save(m, nil); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = OpenStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restored := New(7)
	if err := store.Load(restored); err != nil {
		t.Fatal(err)
	}
	e.Timestamp = now.Add(2 * time.Second)
	apply(t, restored, e)
	if len(restored.Snapshot().NeedsYou) != 0 || restored.Sessions["claude:a"].DismissedAt == nil {
		t.Fatal("restart resurrected dismissal")
	}
}

func TestV04SessionJSONRetainsMetadataWithoutInventingNativeID(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := New(7)
	apply(t, m, fixture("legacy", event.Done, time.Now()))
	s := m.Sessions["claude:legacy"]
	s.Title = "Legacy title"
	s.Summary = "Legacy summary"
	m.Sessions[s.ID] = s
	if err := store.Save(m, nil); err != nil {
		t.Fatal(err)
	}
	// Remove the additive v0.5 fields to emulate the unchanged v1 row format.
	if err := store.exec(`UPDATE sessions SET session_json=json_remove(session_json,'$.native_session_id','$.runtime_state','$.agent_flavor','$.recovery_capability')`); err != nil {
		t.Fatal(err)
	}
	restored := New(7)
	if err := store.Load(restored); err != nil {
		t.Fatal(err)
	}
	got := restored.Sessions[s.ID]
	if got.Title != s.Title || got.Summary != s.Summary || got.CWD != s.CWD || got.NativeSessionID != "" || got.RuntimeState != RuntimeUnknown {
		t.Fatal(got)
	}
}
