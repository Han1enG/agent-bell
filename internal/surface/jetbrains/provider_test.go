package jetbrains

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

func TestRealBridgeDetectionFocusAndExpiry(t *testing.T) {
	// macOS Unix socket paths have a 104-byte limit; testing.TempDir's
	// descriptive test name can exceed it before adding the UUID filename.
	dir, err := os.MkdirTemp("/private/tmp", "ab-jb-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	instance := "00000000-0000-4000-8000-000000000001"
	id := instance + ":00000000-0000-4000-8000-000000000002"
	listener, err := net.Listen("unix", filepath.Join(dir, instance+".sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var mu sync.Mutex
	live := true
	focused := ""
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				var request map[string]string
				_ = json.NewDecoder(conn).Decode(&request)
				mu.Lock()
				defer mu.Unlock()
				if request["operation"] == "list" {
					contexts := []Context{}
					if live {
						contexts = append(contexts, Context{ContextID: id, Title: "same title"})
					}
					_ = json.NewEncoder(conn).Encode(map[string]any{"ok": true, "contexts": contexts})
				} else {
					ok := live && request["context"] == id
					if ok {
						focused = id
					}
					_ = json.NewEncoder(conn).Encode(map[string]bool{"ok": ok})
				}
			}()
		}
	}()
	p := Provider{Directory: dir, Run: func(name string, args ...string) ([]byte, error) {
		if name == "/bin/ps" {
			return []byte("1 /Users/test/GoLand.app/Contents/MacOS/goland"), nil
		}
		if name == "/usr/libexec/PlistBuddy" {
			return []byte("com.jetbrains.goland"), nil
		}
		return nil, errors.New("unexpected")
	}}
	target, err := p.Detect(surface.DetectContext{PID: 100, CWD: "/same", Env: func(key string) string {
		switch key {
		case "AGENTBELL_SURFACE":
			return "jetbrains"
		case "AGENTBELL_CONTEXT_ID":
			return id
		}
		return ""
	}})
	if err != nil || target.Capability != surface.ReturnExactContext || target.AppBundleID != "com.jetbrains.goland" {
		t.Fatal(target, err)
	}
	if err = p.Probe(*target); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if focused != "" {
		t.Fatal("Probe focused context")
	}
	mu.Unlock()
	if err = p.Return(*target); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if focused != id {
		t.Fatal("wrong tab", focused)
	}
	live = false
	mu.Unlock()
	if err = p.Return(*target); err == nil {
		t.Fatal("focused closed tab")
	}
	var activated bool
	generic := surface.GenericProvider{Run: func(name string, args ...string) ([]byte, error) {
		activated = name == "/usr/bin/open" && reflect.DeepEqual(args, []string{"-b", "com.jetbrains.goland"})
		return nil, nil
	}}
	if err = (surface.Manager{Providers: []surface.SurfaceProvider{p, generic}}).ReturnToContext(*target); err != nil || !activated {
		t.Fatal("missing fallback", err)
	}
	for _, bad := range []string{"../../path:id", instance + ":bad", id + ":extra"} {
		target.ContextID = bad
		if p.Return(*target) == nil {
			t.Fatal("accepted invalid id", bad)
		}
	}
}
