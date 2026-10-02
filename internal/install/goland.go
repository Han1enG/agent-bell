package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	jetbrainsassets "github.com/han1eng/agent-bell/integrations/agentbell-jetbrains"
)

type GoLandIntegration struct {
	HomeDir  string
	AppPaths []string
}
type GoLandStatus struct {
	Detected, Supported, Installed, Managed, Current bool
	Path, Version, Preference                        string
}

var golandDirectory = regexp.MustCompile(`^GoLand[0-9]{4}\.[0-9]+$`)

const golandJar = "lib/agentbell.jar"

func (g GoLandIntegration) preference(value *string) (string, error) {
	path := filepath.Join(g.HomeDir, ".config", "agentbell", "integrations.json")
	state := map[string]string{}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if state == nil {
		state = map[string]string{}
	}
	if value == nil {
		return state["goland"], nil
	}
	if *value == "" {
		delete(state, "goland")
	} else {
		state["goland"] = *value
	}
	if err = safeDirectories(g.HomeDir, filepath.Dir(path)); err != nil {
		return "", err
	}
	data, _ = json.MarshalIndent(state, "", "  ")
	return *value, writeAtomic(path, append(data, '\n'))
}
func (g GoLandIntegration) Status() (GoLandStatus, error) {
	var s GoLandStatus
	var err error
	s.Preference, err = g.preference(nil)
	if err != nil {
		return s, err
	}
	paths := g.AppPaths
	if paths == nil {
		paths = []string{filepath.Join(g.HomeDir, "Applications", "GoLand.app"), "/Applications/GoLand.app"}
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(path, "Contents", "Resources", "product-info.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return s, err
		}
		var product struct{ Version, BuildNumber, DataDirectoryName, ProductCode string }
		if err = json.Unmarshal(data, &product); err != nil {
			return s, err
		}
		if product.ProductCode != "GO" {
			continue
		}
		s.Detected, s.Version = true, product.Version
		s.Supported = strings.HasPrefix(product.BuildNumber, "253.") && golandDirectory.MatchString(product.DataDirectoryName)
		if s.Supported {
			s.Path = filepath.Join(g.HomeDir, "Library", "Application Support", "JetBrains", product.DataDirectoryName, "plugins", "agentbell")
		}
		break
	}
	if s.Path == "" {
		return s, nil
	}
	if err = validateExistingDirectories(g.HomeDir, s.Path); err != nil {
		return s, err
	}
	if _, err = os.Lstat(s.Path); os.IsNotExist(err) {
		return s, nil
	} else if err != nil {
		return s, err
	}
	s.Installed = true
	manifest, err := readGoLandManifest(s.Path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s.Managed = manifest.Owner == "agentbell" && len(manifest.Files) == 1 && manifest.Files[golandJar] != ""
	if s.Managed {
		data, err := readRegular(filepath.Join(s.Path, golandJar))
		s.Current = err == nil && digest(data) == manifest.Files[golandJar] && digest(data) == digest(jetbrainsassets.Jar)
	}
	return s, nil
}
func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("preserving nonregular plugin file %s", path)
	}
	return os.ReadFile(path)
}
func readGoLandManifest(path string) (pluginManifest, error) {
	var m pluginManifest
	data, err := readRegular(filepath.Join(path, tabbyMarker))
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	return m, err
}
func validateGoLandManaged(home, path string) error {
	if err := validateExistingDirectories(home, filepath.Join(path, "lib")); err != nil {
		return err
	}
	m, err := readGoLandManifest(path)
	if err != nil {
		return err
	}
	if m.Owner != "agentbell" || len(m.Files) != 1 || m.Files[golandJar] == "" {
		return errors.New("existing GoLand plugin is not managed; preserved")
	}
	data, err := readRegular(filepath.Join(path, golandJar))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && digest(data) != m.Files[golandJar] {
		return errors.New("GoLand plugin was modified; preserved")
	}
	return nil
}
func (g GoLandIntegration) Install() error {
	s, err := g.Status()
	if err != nil {
		return err
	}
	if !s.Supported {
		return errors.New("GoLand bridge currently supports 2025.3 (build 253) only")
	}
	if s.Installed {
		if err = validateGoLandManaged(g.HomeDir, s.Path); err != nil {
			return err
		}
		// Atomic replacement must not remove any extra user files.
		err = filepath.WalkDir(s.Path, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, _ := filepath.Rel(s.Path, path)
			if rel != "." && rel != "lib" && rel != golandJar && rel != tabbyMarker {
				return fmt.Errorf("preserving unexpected plugin file %s", rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err = safeDirectories(g.HomeDir, filepath.Dir(s.Path)); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(s.Path), ".agentbell-plugin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err = os.Mkdir(filepath.Join(stage, "lib"), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, golandJar), jetbrainsassets.Jar, 0600); err != nil {
		return err
	}
	m := pluginManifest{Owner: "agentbell", Files: map[string]string{golandJar: digest(jetbrainsassets.Jar)}}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, tabbyMarker), data, 0600); err != nil {
		return err
	}
	backup := stage + "-previous"
	if s.Installed {
		if err = os.Rename(s.Path, backup); err != nil {
			return err
		}
	}
	if err = os.Rename(stage, s.Path); err != nil {
		if s.Installed {
			if restoreErr := os.Rename(backup, s.Path); restoreErr != nil {
				return fmt.Errorf("install failed %v; restore failed %v; backup %s", err, restoreErr, backup)
			}
		}
		return err
	}
	if s.Installed {
		if err = os.RemoveAll(backup); err != nil {
			return err
		}
	}
	enabled := "enabled"
	_, err = g.preference(&enabled)
	return err
}
func (g GoLandIntegration) Configure(mode string, output io.Writer) error {
	if mode == "skip" {
		disabled := "disabled"
		_, err := g.preference(&disabled)
		return err
	}
	s, err := g.Status()
	if err != nil {
		return err
	}
	if mode != "enable" && (s.Preference == "disabled" || !s.Detected) {
		return nil
	}
	if s.Current {
		if mode == "enable" {
			enabled := "enabled"
			if _, err = g.preference(&enabled); err != nil {
				return err
			}
		}
		fmt.Fprintln(output, "✓ GoLand integration is up to date")
		return nil
	}
	if err = g.Install(); err != nil {
		return err
	}
	fmt.Fprintln(output, "✓ GoLand integration installed. Restart GoLand and create a new local terminal tab to enable return-to-session.")
	return nil
}
func (g GoLandIntegration) Remove() error {
	paths, err := filepath.Glob(filepath.Join(g.HomeDir, "Library", "Application Support", "JetBrains", "GoLand*", "plugins", "agentbell"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if !golandDirectory.MatchString(filepath.Base(filepath.Dir(filepath.Dir(path)))) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(path, tabbyMarker)); os.IsNotExist(err) {
			continue
		}
		if err = validateGoLandManaged(g.HomeDir, path); err != nil {
			return err
		}
		for _, name := range []string{golandJar, tabbyMarker} {
			if err = os.Remove(filepath.Join(path, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		// Only remove empty directories, preserving unexpected extra files.
		_ = os.Remove(filepath.Join(path, "lib"))
		_ = os.Remove(path)
	}
	empty := ""
	_, err = g.preference(&empty)
	return err
}
