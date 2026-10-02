package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tabbyassets "github.com/han1eng/agent-bell/integrations/agentbell-tabby"
)

const tabbyMarker = ".agentbell-managed.json"

var tabbyFiles = []string{"package.json", "index.js", "bridge.js", "context.js"}

type TabbyIntegration struct {
	HomeDir string
	// nil uses standard macOS application locations; tests can supply locations.
	AppPaths []string
}
type TabbyStatus struct {
	Detected, Installed, Managed, Current bool
	Version, Preference                   string
}
type pluginManifest struct {
	Owner string            `json:"owner"`
	Files map[string]string `json:"files"`
}

func (t TabbyIntegration) PluginPath() string {
	return filepath.Join(t.HomeDir, "Library", "Application Support", "tabby", "plugins", "node_modules", "tabby-agentbell")
}
func (t TabbyIntegration) statePath() string {
	return filepath.Join(t.HomeDir, ".config", "agentbell", "integrations.json")
}
func (t TabbyIntegration) preference() (string, error) {
	data, err := os.ReadFile(t.statePath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var state map[string]string
	if err = json.Unmarshal(data, &state); err != nil {
		return "", err
	}
	return state["tabby"], nil
}
func (t TabbyIntegration) savePreference(value string) error {
	state := map[string]string{}
	if data, err := os.ReadFile(t.statePath()); err == nil {
		if err = json.Unmarshal(data, &state); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if state == nil {
		state = map[string]string{}
	}
	if value == "" {
		delete(state, "tabby")
	} else {
		state["tabby"] = value
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err = safeDirectories(t.HomeDir, filepath.Dir(t.statePath())); err != nil {
		return err
	}
	return writeAtomic(t.statePath(), append(data, '\n'))
}
func (t TabbyIntegration) Status() (TabbyStatus, error) {
	var status TabbyStatus
	if err := validateExistingDirectories(t.HomeDir, filepath.Dir(t.PluginPath())); err != nil {
		return status, err
	}
	paths := t.AppPaths
	if paths == nil {
		paths = []string{"/Applications/Tabby.app", filepath.Join(t.HomeDir, "Applications", "Tabby.app")}
	}
	for _, path := range paths {
		if info, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err == nil && !info.IsDir() {
			status.Detected = true
			break
		}
	}
	var err error
	status.Preference, err = t.preference()
	if err != nil {
		return status, fmt.Errorf("read integration preference: %w", err)
	}
	info, err := os.Lstat(t.PluginPath())
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.Installed = true
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return status, errors.New("Tabby integration path must be a real directory")
	}
	data, err := os.ReadFile(filepath.Join(t.PluginPath(), "package.json"))
	if err == nil {
		var pkg struct{ Version string }
		if json.Unmarshal(data, &pkg) == nil {
			status.Version = pkg.Version
		}
	}
	manifest, err := t.manifest()
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.Managed = manifest.Owner == "agentbell" && len(manifest.Files) == len(tabbyFiles)
	for _, name := range tabbyFiles {
		if manifest.Files[name] == "" {
			status.Managed = false
		}
	}
	if status.Managed {
		status.Current = t.matchesBundled()
	}
	return status, nil
}
func (t TabbyIntegration) manifest() (pluginManifest, error) {
	var manifest pluginManifest
	path := filepath.Join(t.PluginPath(), tabbyMarker)
	info, err := os.Lstat(path)
	if err != nil {
		return manifest, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return manifest, errors.New("integration marker must not be a symlink")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func (t TabbyIntegration) matchesBundled() bool {
	for _, name := range tabbyFiles {
		path := filepath.Join(t.PluginPath(), name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		got, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		want, _ := tabbyassets.Files.ReadFile(name)
		if digest(got) != digest(want) {
			return false
		}
	}
	return true
}

// Install only replaces a verified managed integration, or adopts an exact copy
// of the bundled plugin. Unrelated plugins and modified files are preserved.
func (t TabbyIntegration) Install() error {
	if t.HomeDir == "" {
		return errors.New("home directory is required")
	}
	if err := safeDirectories(t.HomeDir, filepath.Dir(t.PluginPath())); err != nil {
		return err
	}
	status, err := t.Status()
	if err != nil {
		return err
	}
	if status.Installed {
		entries, err := os.ReadDir(t.PluginPath())
		if err != nil {
			return err
		}
		allowed := map[string]bool{tabbyMarker: true}
		for _, name := range tabbyFiles {
			allowed[name] = true
		}
		for _, entry := range entries {
			if !allowed[entry.Name()] {
				return fmt.Errorf("preserving unexpected file in integration: %s", entry.Name())
			}
		}
		if !status.Managed {
			if !t.matchesBundled() {
				return errors.New("existing tabby-agentbell is not managed by AgentBell; it was preserved")
			}
		} else {
			manifest, _ := t.manifest()
			for _, name := range tabbyFiles {
				info, err := os.Lstat(filepath.Join(t.PluginPath(), name))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("preserving nonregular plugin file %s", name)
				}
				data, err := os.ReadFile(filepath.Join(t.PluginPath(), name))
				if err != nil {
					return err
				}
				if digest(data) != manifest.Files[name] {
					return fmt.Errorf("plugin file %s was modified; preserved", name)
				}
			}
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(t.PluginPath()), ".agentbell-plugin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	manifest := pluginManifest{Owner: "agentbell", Files: map[string]string{}}
	for _, name := range tabbyFiles {
		data, err := tabbyassets.Files.ReadFile(name)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
			return err
		}
		manifest.Files[name] = digest(data)
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, tabbyMarker), data, 0600); err != nil {
		return err
	}
	// Remember the enabled integration; a failed install can be repaired.
	if err = t.savePreference("enabled"); err != nil {
		return err
	}
	backup := stage + "-previous"
	if status.Installed {
		if err = os.Rename(t.PluginPath(), backup); err != nil {
			return err
		}
	}
	if err = os.Rename(stage, t.PluginPath()); err != nil {
		if status.Installed {
			if restoreErr := os.Rename(backup, t.PluginPath()); restoreErr != nil {
				return fmt.Errorf("install failed: %v; restore failed: %w; backup at %s", err, restoreErr, backup)
			}
		}
		return err
	}
	if status.Installed {
		if err = os.RemoveAll(backup); err != nil {
			return fmt.Errorf("plugin installed but previous copy cleanup failed: %w", err)
		}
	}
	return nil
}
func (t TabbyIntegration) Remove() error {
	status, err := t.Status()
	if err != nil {
		return err
	}
	if status.Managed {
		manifest, _ := t.manifest()
		// Validate everything before removing anything; never erase local edits.
		for _, name := range tabbyFiles {
			path := filepath.Join(t.PluginPath(), name)
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || digest(data) != manifest.Files[name] {
				return fmt.Errorf("plugin file %s was modified; preserved", name)
			}
		}
		for _, name := range append(append([]string{}, tabbyFiles...), tabbyMarker) {
			if err = os.Remove(filepath.Join(t.PluginPath(), name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		// Keep any extra files; Remove never recurses through another plugin.
		entries, err := os.ReadDir(t.PluginPath())
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err = os.Remove(t.PluginPath()); err != nil {
				return err
			}
		}
	}
	if _, err := os.Stat(t.statePath()); os.IsNotExist(err) {
		return nil
	}
	return t.savePreference("")
}

// Configure installs automatically when Tabby is detected. It never restarts
// the application. --skip-tabby is a persistent opt-out, --tabby enables it again.
func (t TabbyIntegration) Configure(mode string, output io.Writer) error {
	if mode == "skip" {
		if err := t.savePreference("disabled"); err != nil {
			return err
		}
		fmt.Fprintln(output, "○ Tabby integration skipped; an existing integration is preserved")
		return nil
	}
	status, err := t.Status()
	if err != nil {
		return err
	}
	if mode != "enable" {
		if status.Preference == "disabled" || status.Preference == "declined" {
			fmt.Fprintln(output, "○ Tabby integration skipped (your preference; enable with agentbell install --tabby)")
			return nil
		}
		if status.Current && status.Preference == "enabled" {
			fmt.Fprintln(output, "✓ Tabby integration is up to date")
			return nil
		}
		if !status.Detected && status.Preference != "enabled" {
			return nil
		}
	}
	if err = t.Install(); err != nil {
		return err
	}
	fmt.Fprintln(output, "✓ Tabby integration installed/updated. 下次启动 Tabby 后生效；请在新本地 Tab 中启动 agent。")
	return nil
}
func safeDirectories(home, path string) error {
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("integration path is outside home")
	}
	current := home
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err = os.Mkdir(current, 0700); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("expected real directory: %s", current)
		}
	}
	return nil
}
func writeAtomic(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("expected regular file: %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".agentbell-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func validateExistingDirectories(home, path string) error {
	if home == "" {
		return errors.New("home directory is required")
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("integration path is outside home")
	}
	current := home
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("expected real directory: %s", current)
		}
	}
	return nil
}
