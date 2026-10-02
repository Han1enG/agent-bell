package tabby

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

func TestUnavailableBridgeFallsBackWithoutGuessing(t *testing.T) {
	for _, mode := range []string{"missing", "non-socket", "invalid-id"} {
		t.Run(mode, func(t *testing.T) {
			dir, err := os.MkdirTemp("/private/tmp", "ab-f-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			window := "12345678-1234-1234-1234-123456789abc"
			id := window + ":aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
			if mode == "non-socket" {
				if err := os.WriteFile(filepath.Join(dir, window+".sock"), []byte("crashed bridge"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := surface.BridgeUnreachable
			if mode == "invalid-id" {
				id = "../escape:bad"
				want = surface.InvalidTarget
			}
			activated := false
			p := Provider{Directory: dir}
			generic := surface.GenericProvider{Run: func(name string, args ...string) ([]byte, error) {
				if name != "/usr/bin/open" || len(args) != 2 || args[0] != "-b" || args[1] != "org.tabby" {
					return nil, errors.New("unexpected fallback")
				}
				activated = true
				return nil, nil
			}}
			var reasons []surface.FailureReason
			err = (surface.Manager{Providers: []surface.SurfaceProvider{p, generic}, OnAttempt: func(_ string, cap surface.ReturnCapability, err error) {
				if cap == surface.ReturnExactContext {
					reasons = append(reasons, surface.Reason(err))
				}
			}}).ReturnToContext(surface.ReturnTarget{Surface: "tabby", ContextID: id, AppBundleID: "org.tabby", Capability: surface.ReturnExactContext})
			if err != nil || !activated || len(reasons) != 1 || reasons[0] != want {
				t.Fatal(err, activated, reasons)
			}
		})
	}
}
