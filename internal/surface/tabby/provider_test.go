package tabby

import (
	"encoding/json"
	"fmt"
	"github.com/han1eng/agent-bell/internal/surface"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestBridgeDetectionFocusAndExpiry(t *testing.T) {
	dir, err := os.MkdirTemp("/private/tmp", "abt-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	window := "12345678-1234-1234-1234-123456789abc"
	contextID := window + ":aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	listener, err := net.Listen("unix", filepath.Join(dir, window+".sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var focuses atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				var req map[string]string
				if json.NewDecoder(conn).Decode(&req) != nil {
					return
				}
				if req["operation"] == "list" {
					contexts := []Context{{ContextID: contextID, Title: "B"}}
					for i := 1; i < 100; i++ {
						contexts = append(contexts, Context{ContextID: window + fmt.Sprintf(":%08x-aaaa-aaaa-aaaa-%012x", i, i), Title: "same cwd"})
					}
					_ = json.NewEncoder(conn).Encode(map[string]any{"contexts": contexts})
				} else {
					focuses.Add(1)
					_ = json.NewEncoder(conn).Encode(map[string]bool{"ok": req["context"] == contextID})
				}
			}()
		}
	}()
	p := Provider{Directory: dir}
	targetForProbe := surface.ReturnTarget{Surface: "tabby", ContextID: contextID, Capability: surface.ReturnExactContext}
	if err := p.Probe(targetForProbe); err != nil || focuses.Load() != 0 {
		t.Fatal("probe focused or failed", err)
	}
	contexts, err := p.List(window)
	if err != nil || len(contexts) != 100 {
		t.Fatal("100-context list failed", err, len(contexts))
	}
	if err := p.Return(surface.ReturnTarget{ContextID: contextID}); err != nil {
		t.Fatal(err)
	}
	if p.Return(surface.ReturnTarget{ContextID: window + ":bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}) == nil {
		t.Fatal("expired context accepted")
	}
	for _, bad := range []string{"../escape:bad", contextID + ";touch bad", ""} {
		if p.Return(surface.ReturnTarget{ContextID: bad}) == nil {
			t.Fatal("invalid id accepted")
		}
	}
	target, err := p.Detect(surface.DetectContext{Env: func(key string) string {
		switch key {
		case "AGENTBELL_SURFACE":
			return "tabby"
		case "AGENTBELL_CONTEXT_ID":
			return contextID
		}
		return ""
	}})
	if err != nil || target.Capability != surface.ReturnExactContext {
		t.Fatal(target, err)
	}
}
