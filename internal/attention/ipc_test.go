package attention

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shortHome(t *testing.T) string {
	t.Helper()
	p, e := os.MkdirTemp("/tmp", "ab-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(p) })
	return p
}
func TestIPCVersionOwnershipAndUnknownFields(t *testing.T) {
	home := shortHome(t)
	l, cleanup, err := Listen(home)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if other, clean, err := Listen(home); err == nil {
		other.Close()
		clean()
		t.Fatal("second host replaced live socket")
	}
	info, _ := os.Stat(SocketPath(home))
	if info.Mode().Perm() != 0600 {
		t.Fatal("socket permissions")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, l, func(r Request) Response { v := New(7).Snapshot(); return Response{State: &v} })
	}()
	if _, err = RequestTo(SocketPath(home), Request{Version: 2, Command: "status"}, time.Second); err == nil {
		t.Fatal("major version accepted")
	}
	r, err := RequestTo(SocketPath(home), Request{Version: 1, Command: "status"}, time.Second)
	if err != nil || r.State == nil || r.State.SchemaVersion != 1 {
		t.Fatal(r, err)
	}
	conn, err := net.Dial("unix", SocketPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, _ = conn.Write([]byte("{\"version\":1,\"command\":\"status\",\"future\":true}\n"))
	b := make([]byte, 4096)
	if _, err = conn.Read(b); err != nil {
		t.Fatal("unknown fields not ignored", err)
	}
	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
func TestIPCBoundedTimeout(t *testing.T) {
	home := shortHome(t)
	path := filepath.Join(home, "wedged.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_ = os.Chmod(path, 0600)
	start := time.Now()
	if _, err = RequestTo(path, Request{Version: 1, Command: "status"}, HookTimeout); err == nil {
		t.Fatal("wedged host succeeded")
	}
	if time.Since(start) > 150*time.Millisecond {
		t.Fatal("hook request blocked too long")
	}
	if _, err = RequestTo(filepath.Join(home, "missing.sock"), Request{Version: 1}, HookTimeout); err == nil {
		t.Fatal("missing host succeeded")
	}
}
