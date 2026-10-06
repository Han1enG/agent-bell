package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/han1eng/agent-bell/internal/config"
	"github.com/han1eng/agent-bell/internal/install"
	"github.com/han1eng/agent-bell/internal/notify"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func installedAppVersion(home, executable string) string {
	paths := []string{filepath.Join(home, "Applications", "AgentBell.app", "Contents", "Info.plist")}
	if helper := notify.NativeHelperFor(executable); helper != "" {
		if resolved, err := filepath.EvalSymlinks(helper); err == nil {
			paths = append(paths, filepath.Join(filepath.Dir(filepath.Dir(resolved)), "Info.plist"))
		}
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		d := xml.NewDecoder(f)
		key := ""
		value := ""
		release := ""
		for {
			token, err := d.Token()
			if err != nil {
				break
			}
			if start, ok := token.(xml.StartElement); ok {
				switch start.Name.Local {
				case "key":
					_ = d.DecodeElement(&key, &start)
				case "string":
					if key == "AgentBellReleaseVersion" {
						_ = d.DecodeElement(&release, &start)
					}
					if key == "CFBundleShortVersionString" {
						_ = d.DecodeElement(&value, &start)
					}
				}
			}

		}
		f.Close()
		if release != "" {
			return release
		}
		if value != "" {
			return value
		}
	}
	return "not installed"
}
func openConfig() error {
	home, _ := os.UserHomeDir()
	path := config.Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		_, err = io.WriteString(f, "[attention_center]\nenabled = true\nlaunch_at_login = true\nretention_days = 7\nrecent_limit = 5\n\n[notifications]\ndone = true\nneeds_input = true\nneeds_approval = true\nerror = true\npermission_request = false\n\n[return]\nenabled = true\nfallback_app = \"auto\"\n")
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	} else if !os.IsExist(err) {
		return err
	}
	return exec.Command("/usr/bin/open", path).Run()
}
func logsCommand(args []string, out io.Writer) error {
	follow := len(args) == 1 && args[0] == "--follow"
	if len(args) > 0 && !follow {
		return errors.New("usage: agentbell logs [--follow]")
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "Logs", "AgentBell", "agentbell.log")
	for {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		lines := []string{}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 65536)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
			if len(lines) > 100 {
				lines = lines[1:]
			}
		}
		err = scanner.Err()
		offset, _ := f.Seek(0, io.SeekCurrent)
		f.Close()
		if err != nil {
			return err
		}
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
		if !follow {
			return nil
		}
		for {
			time.Sleep(250 * time.Millisecond)
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			info, _ := f.Stat()
			if info.Size() < offset {
				offset = 0
			}
			_, err = f.Seek(offset, io.SeekStart)
			if err == nil {
				_, err = io.Copy(out, f)
				offset, _ = f.Seek(0, io.SeekCurrent)
			}
			f.Close()
			if err != nil {
				return err
			}
		}
	}
}
func printAppInstallation(home, executable string, out io.Writer) {
	appVersion := installedAppVersion(home, executable)
	fmt.Fprintf(out, "App installed: %s\n", appVersion)
	if appVersion != "not installed" && appVersion != version {
		fmt.Fprintln(out, "○ Installed CLI/App version mismatch")
	}
	cfg, err := config.Load(config.Path(home))
	if err == nil && cfg.AttentionCenter.Enabled && cfg.AttentionCenter.LaunchAtLogin {
		if _, err := os.Stat(install.AttentionLoginPath(home)); err == nil {
			fmt.Fprintln(out, "✓ Login launch configured")
		} else {
			fmt.Fprintln(out, "○ Login launch missing (agentbell install / doctor --fix)")
		}
	}
}

// This runs only the locally packaged app's fixed administrative action.
func quitPackagedAttention(executable string) {
	if helper := notify.NativeHelperFor(executable); helper != "" {
		if resolved, err := filepath.EvalSymlinks(helper); err == nil {
			native := filepath.Join(filepath.Dir(resolved), "AgentBellApp")
			if _, err = os.Stat(native); err == nil {
				if err = exec.Command(native, "--quit-existing").Run(); err != nil {
					writeDebugLog("app_shutdown_error=%v", err)
				}
			}
		}
	}
}
