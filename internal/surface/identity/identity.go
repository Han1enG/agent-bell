// Package identity pins local socket and process generations without inspecting content.
package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/han1eng/agent-bell/internal/surface"
)

func Socket(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", surface.Fail(surface.InvalidTarget, errors.New("invalid socket path"))
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", surface.Fail(surface.BridgeUnreachable, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return "", surface.Fail(surface.InvalidTarget, errors.New("not a Unix socket"))
	}
	stat := reflect.Indirect(reflect.ValueOf(info.Sys()))
	if !stat.IsValid() || stat.FieldByName("Uid").Uint() != uint64(os.Getuid()) {
		return "", surface.Fail(surface.PermissionDenied, errors.New("socket owned by another user"))
	}
	// Device, inode and ctime (including nanoseconds) invalidate same-path restarts.
	ctime := stat.FieldByName("Ctimespec")
	if !ctime.IsValid() {
		ctime = stat.FieldByName("Ctim")
	}
	if !ctime.IsValid() {
		return "", surface.Fail(surface.UnsupportedSurface, errors.New("socket generation unavailable"))
	}
	return fmt.Sprintf("%v:%v:%v", stat.FieldByName("Dev"), stat.FieldByName("Ino"), ctime.Interface()), nil
}
