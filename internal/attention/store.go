//go:build cgo

package attention

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
static int ab_bind(sqlite3_stmt *s, int i, const char *v) { return sqlite3_bind_text(s,i,v,-1,SQLITE_TRANSIENT); }
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

type Store struct {
	db       *C.sqlite3
	ReadOnly bool
}

func OpenStore(path string, readOnly bool) (*Store, error) {
	if !readOnly {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return nil, errors.New("unsafe store path")
	}
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE)
	if readOnly {
		flags = C.SQLITE_OPEN_READONLY
	}
	s := &Store{ReadOnly: readOnly}
	if C.sqlite3_open_v2(p, &s.db, flags, nil) != C.SQLITE_OK {
		err := s.err()
		s.Close()
		return nil, err
	}
	C.sqlite3_busy_timeout(s.db, 100)
	if !readOnly {
		if err := os.Chmod(path, 0600); err != nil {
			s.Close()
			return nil, err
		}
	}
	version, err := s.scalar("PRAGMA user_version")
	if err != nil {
		s.Close()
		return nil, err
	}
	if version != "0" && version != "1" {
		s.Close()
		return nil, fmt.Errorf("unsupported SQLite schema %s", version)
	}
	if version == "0" {
		if readOnly {
			s.Close()
			return nil, errors.New("uninitialized SQLite schema")
		}
		err = s.exec(`BEGIN IMMEDIATE;
CREATE TABLE sessions(id TEXT PRIMARY KEY, agent TEXT NOT NULL, project TEXT NOT NULL, cwd TEXT NOT NULL, status TEXT NOT NULL, attention TEXT NOT NULL, summary TEXT NOT NULL, return_target_json TEXT, started_at TEXT NOT NULL, updated_at TEXT NOT NULL, finished_at TEXT, last_event_type TEXT NOT NULL, session_json TEXT NOT NULL);
CREATE TABLE events(id INTEGER PRIMARY KEY, session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE, type TEXT NOT NULL, timestamp TEXT NOT NULL);
CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
PRAGMA user_version=1;
COMMIT;`)
		if err != nil {
			s.Close()
			return nil, err
		}
	}
	if err = s.exec("PRAGMA foreign_keys=ON"); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() {
	if s != nil && s.db != nil {
		C.sqlite3_close(s.db)
		s.db = nil
	}
}
func (s *Store) err() error { return fmt.Errorf("SQLite: %s", C.GoString(C.sqlite3_errmsg(s.db))) }
func (s *Store) exec(sql string) error {
	q := C.CString(sql)
	defer C.free(unsafe.Pointer(q))
	if C.sqlite3_exec(s.db, q, nil, nil, nil) != C.SQLITE_OK {
		return s.err()
	}
	return nil
}
func (s *Store) prepare(sql string) (*C.sqlite3_stmt, error) {
	q := C.CString(sql)
	defer C.free(unsafe.Pointer(q))
	var st *C.sqlite3_stmt
	if C.sqlite3_prepare_v2(s.db, q, -1, &st, nil) != C.SQLITE_OK {
		return nil, s.err()
	}
	return st, nil
}
func (s *Store) scalar(sql string) (string, error) {
	st, err := s.prepare(sql)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(st)
	r := C.sqlite3_step(st)
	if r == C.SQLITE_DONE {
		return "", nil
	}
	if r != C.SQLITE_ROW {
		return "", s.err()
	}
	return C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(st, 0)))), nil
}
func (s *Store) put(sql string, args ...string) error {
	st, err := s.prepare(sql)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(st)
	for i, v := range args {
		p := C.CString(v)
		r := C.ab_bind(st, C.int(i+1), p)
		C.free(unsafe.Pointer(p))
		if r != C.SQLITE_OK {
			return s.err()
		}
	}
	if C.sqlite3_step(st) != C.SQLITE_DONE {
		return s.err()
	}
	return nil
}
func (s *Store) Load(m *Engine) error {
	st, err := s.prepare("SELECT session_json FROM sessions")
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(st)
	for {
		r := C.sqlite3_step(st)
		if r == C.SQLITE_DONE {
			break
		}
		if r != C.SQLITE_ROW {
			return s.err()
		}
		var v Session
		if err = json.Unmarshal([]byte(C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(st, 0))))), &v); err != nil {
			return err
		}
		// Older snapshots lack a turn start. Recover only a recorded real transition.
		if v.Status == SessionWorking && v.WorkingAt == nil {
			if at, e := s.scalar("SELECT timestamp FROM events WHERE session_id='" + strings.ReplaceAll(v.ID, "'", "''") + "' AND type IN ('working','session_started') ORDER BY id DESC LIMIT 1"); e == nil && at != "" {
				if t, e := time.Parse(time.RFC3339Nano, at); e == nil {
					v.WorkingAt = &t
				}
			}
		}
		if v.RuntimeState == "" {
			v.RuntimeState = RuntimeUnknown
		}
		if v.AgentFlavor == "" {
			v.AgentFlavor = "unknown"
		}
		if v.RecoveryCapability == "" {
			v.RecoveryCapability = "unknown"
		}
		m.Sessions[v.ID] = v
	}
	paused, err := s.scalar("SELECT value FROM settings WHERE key='paused'")
	m.Paused = paused == "true"
	return err
}

type Transition struct{ ID, Type, Timestamp string }

// Save commits snapshots and a bounded transition journal atomically. Tool activity
// is coalesced by the host and never enters this journal.
func (s *Store) Save(m *Engine, events []Transition) (err error) {
	if s.ReadOnly {
		return errors.New("read-only store")
	}
	if err = s.exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = s.exec("ROLLBACK")
		}
	}()
	// Remove only sessions no longer retained. Keep the journal for live sessions.
	st, e := s.prepare("SELECT id FROM sessions")
	if e != nil {
		return e
	}
	var removed []string
	for {
		r := C.sqlite3_step(st)
		if r == C.SQLITE_DONE {
			break
		}
		if r != C.SQLITE_ROW {
			C.sqlite3_finalize(st)
			return s.err()
		}
		id := C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(st, 0))))
		if _, ok := m.Sessions[id]; !ok {
			removed = append(removed, id)
		}
	}
	C.sqlite3_finalize(st)
	for _, id := range removed {
		if err = s.put("DELETE FROM sessions WHERE id=?", id); err != nil {
			return err
		}
	}
	for _, v := range m.Sessions {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		target, _ := json.Marshal(v.ReturnTarget)
		finished := ""
		if v.FinishedAt != nil {
			finished = v.FinishedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
		}
		err = s.put(`INSERT INTO sessions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET agent=excluded.agent,project=excluded.project,cwd=excluded.cwd,status=excluded.status,attention=excluded.attention,summary=excluded.summary,return_target_json=excluded.return_target_json,started_at=excluded.started_at,updated_at=excluded.updated_at,finished_at=excluded.finished_at,last_event_type=excluded.last_event_type,session_json=excluded.session_json`, v.ID, v.Agent, v.Project, v.CWD, string(v.Status), string(v.Attention), v.Summary, string(target), v.StartedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), v.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), finished, v.LastEventType, string(b))
		if err != nil {
			return err
		}
	}
	for _, e := range events {
		if _, ok := m.Sessions[e.ID]; ok {
			if err = s.put("INSERT INTO events(session_id,type,timestamp) VALUES(?,?,?)", e.ID, e.Type, e.Timestamp); err != nil {
				return err
			}
		}
	}
	if err = s.exec("DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY id DESC LIMIT 200)"); err != nil {
		return err
	}
	if err = s.put("INSERT INTO settings VALUES('paused',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", fmt.Sprint(m.Paused)); err != nil {
		return err
	}
	return s.exec("COMMIT")
}
func (s *Store) Health() error {
	v, err := s.scalar("PRAGMA quick_check")
	if err == nil && v != "ok" {
		return errors.New(v)
	}
	return err
}
