package install

import (
	"encoding/xml"
	"fmt"
	"github.com/han1eng/agent-bell/internal/notify"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const AttentionLoginLabel = "com.agentbell.attentioncenter"

func AttentionLoginPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", AttentionLoginLabel+".plist")
}

// InstallAttentionApp copies only the packaged, trusted bundle. Test fixtures and
// CLI-only distributions with no bundle retain the notification-only path.
func (i Installer) InstallAttentionApp(login bool) (string, error) {
	helper := notify.NativeHelperFor(i.Executable)
	resolved, err := filepath.EvalSymlinks(helper)
	if err != nil {
		return "", nil
	}
	source := filepath.Dir(filepath.Dir(filepath.Dir(resolved)))
	if filepath.Base(source) != "AgentBell.app" {
		return "", nil
	}
	if _, err := os.Stat(filepath.Join(source, "Contents", "MacOS", "AgentBellApp")); err != nil {
		return "", fmt.Errorf("packaged Attention Center missing: %w", err)
	}
	app := filepath.Join(i.HomeDir, "Applications", "AgentBell.app")
	if source != app {
		if err := os.MkdirAll(filepath.Dir(app), 0700); err != nil {
			return "", err
		}
		// Stage an entire signed bundle before replacing a previous AgentBell bundle.
		stage, err := os.MkdirTemp(filepath.Dir(app), ".AgentBell-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(stage)
		staged := filepath.Join(stage, "AgentBell.app")
		if err = copyBundle(source, staged); err != nil {
			return "", err
		}
		if info, e := os.Lstat(app); e == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("refusing to replace symlink at %s", app)
			}
			data, e := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
			if e != nil || !strings.Contains(string(data), "com.agentbell.AgentBell") {
				return "", fmt.Errorf("refusing to replace unrelated app at %s", app)
			}
		}
		backup := filepath.Join(stage, "previous.app")
		if _, e := os.Stat(app); e == nil {
			if err = os.Rename(app, backup); err != nil {
				return "", err
			}
		}
		if err = os.Rename(staged, app); err != nil {
			_ = os.Rename(backup, app)
			return "", err
		}
	}
	path := AttentionLoginPath(i.HomeDir)
	if !login {
		if data, e := os.ReadFile(path); e == nil && strings.Contains(string(data), AttentionLoginLabel) {
			_ = os.Remove(path)
		}
		return app, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	var escaped strings.Builder
	if err := xml.EscapeText(&escaped, []byte(app)); err != nil {
		return "", err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>/usr/bin/open</string><string>-g</string><string>%s</string></array><key>RunAtLoad</key><true/><key>LimitLoadToSessionType</key><string>Aqua</string></dict></plist>
`, AttentionLoginLabel, escaped.String())
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	return app, nil
}
func copyBundle(source, dest string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(to, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported packaged bundle file %s", rel)
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		closeErr := dst.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func RemoveAttentionLogin(home string) error {
	path := AttentionLoginPath(home)
	if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), AttentionLoginLabel) {
		return os.Remove(path)
	}
	return nil
}
