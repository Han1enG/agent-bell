package attention

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/surface/identity"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrIncompatible = errors.New("incompatible Attention Center protocol")

const ProtocolVersion = 1
const HookTimeout = 40 * time.Millisecond

func Directory(home string) string {
	return filepath.Join(home, "Library", "Application Support", "AgentBell")
}
func SocketPath(home string) string { return filepath.Join(Directory(home), "agentbell.sock") }

// DisabledPath records deliberate user Quit, never a crash or upgrade shutdown.
func DisabledPath(home string) string { return filepath.Join(Directory(home), "user-disabled") }
func UserDisabled(home string) bool   { _, err := os.Stat(DisabledPath(home)); return err == nil }
func DBPath(home string) string       { return filepath.Join(Directory(home), "agentbell.db") }

type Request struct {
	CWD       string            `json:"cwd,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Version   int               `json:"version"`
	Command   string            `json:"command,omitempty"`
	Event     *event.AgentEvent `json:"event,omitempty"`
}
type Response struct {
	Session  *Session      `json:"session,omitempty"`
	Recovery *ResumeResult `json:"recovery,omitempty"`
	Version  int           `json:"version"`
	Error    string        `json:"error,omitempty"`
	State    *Snapshot     `json:"state,omitempty"`
}

// RequestTo uses one deadline for connect, send and acknowledgement. No GUI launch.
func RequestTo(path string, r Request, timeout time.Duration) (Response, error) {
	var v Response
	if _, err := identity.Socket(path); err != nil {
		return v, err
	}
	d := time.Now().Add(timeout)
	conn, err := net.DialTimeout("unix", path, time.Until(d))
	if err != nil {
		return v, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(d)
	if err = json.NewEncoder(conn).Encode(r); err != nil {
		return v, err
	}
	if err = json.NewDecoder(&limitedReader{r: conn, n: 4 << 20}).Decode(&v); err != nil {
		return v, err
	}
	if v.Version != ProtocolVersion {
		return v, ErrIncompatible
	}
	if v.Error != "" {
		if strings.HasPrefix(v.Error, "unsupported IPC version") {
			return v, fmt.Errorf("%w: %s", ErrIncompatible, v.Error)
		}
		return v, errors.New(v.Error)
	}
	return v, nil
}

type limitedReader struct {
	r net.Conn
	n int
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errors.New("IPC response too large")
	}
	if len(p) > l.n {
		p = p[:l.n]
	}
	n, e := l.r.Read(p)
	l.n -= n
	return n, e
}

// Listen locks the app-owned endpoint; a second instance cannot unlink a live host.
func Listen(home string) (net.Listener, func(), error) {
	dir := Directory(home)
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("unsafe Attention Center directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, nil, err
	}
	lockPath := filepath.Join(dir, "host.lock")
	if info, err := os.Lstat(lockPath); err == nil && !info.Mode().IsRegular() {
		return nil, nil, errors.New("unsafe host lock")
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, nil, errors.New("Attention Center already running")
	}
	path := SocketPath(home)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			lock.Close()
			return nil, nil, errors.New("unsafe IPC path")
		}
		if _, err = identity.Socket(path); err != nil {
			lock.Close()
			return nil, nil, err
		}
		if err = os.Remove(path); err != nil {
			lock.Close()
			return nil, nil, err
		}
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		lock.Close()
		return nil, nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		lock.Close()
		return nil, nil, err
	}
	cleanup := func() {
		listener.Close()
		_ = os.Remove(path)
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		lock.Close()
	}
	return listener, cleanup, nil
}

// Serve bounds both readers and concurrency, so a wedged client never blocks hooks.
func Serve(ctx context.Context, l net.Listener, handle func(Request) Response) error {
	go func() { <-ctx.Done(); l.Close() }()
	slots := make(chan struct{}, 16)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			reader := bufio.NewReaderSize(conn, 65537)
			line, err := reader.ReadSlice('\n')
			if err != nil || len(line) > 65536 {
				return
			}
			var r Request
			v := Response{Version: ProtocolVersion}
			if err = json.Unmarshal(line, &r); err != nil {
				v.Error = "invalid IPC request"
			} else if r.Version != ProtocolVersion {
				v.Error = fmt.Sprintf("unsupported IPC version %d", r.Version)
			} else {
				if r.Command == "recovery" {
					_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				}
				v = handle(r)
				v.Version = ProtocolVersion
			}
			_ = json.NewEncoder(conn).Encode(v)
		}()
	}
}
