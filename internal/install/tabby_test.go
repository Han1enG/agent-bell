package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func integrationFixture(t *testing.T, detected bool) TabbyIntegration {
	t.Helper()
	home := t.TempDir()
	app := filepath.Join(home, "Applications", "Tabby.app")
	if detected {
		path := filepath.Join(app, "Contents", "Info.plist")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("plist"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return TabbyIntegration{HomeDir: home, AppPaths: []string{app}}
}
func TestTabbyAutomaticInstallAndRepeat(t *testing.T) {
	integration := integrationFixture(t, true)
	var output bytes.Buffer
	if err := integration.Configure("auto", &output); err != nil {
		t.Fatal(err)
	}
	status, err := integration.Status()
	if err != nil || !status.Detected || !status.Installed || !status.Managed || !status.Current || status.Preference != "enabled" {
		t.Fatal(status, err)
	}
	first := output.String()
	output.Reset()
	if err := integration.Configure("auto", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "up to date") || !strings.Contains(first, "下次启动") {
		t.Fatal(first, output.String())
	}
}
func TestTabbyNoAppSkipsWithoutWriting(t *testing.T) {
	integration := integrationFixture(t, false)
	if err := integration.Configure("auto", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(integration.HomeDir, "Library")); !os.IsNotExist(err) {
		t.Fatal("created plugin directories")
	}
	if _, err := os.Stat(integration.statePath()); !os.IsNotExist(err) {
		t.Fatal("created preferences")
	}
}
func TestTabbyOptOutAndExplicitEnable(t *testing.T) {
	integration := integrationFixture(t, true)
	if err := integration.Configure("skip", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := integration.Configure("auto", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	status, _ := integration.Status()
	if status.Installed || status.Preference != "disabled" {
		t.Fatal(status)
	}
	if err := integration.Configure("enable", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := integration.Configure("skip", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	status, _ = integration.Status()
	if !status.Installed || status.Preference != "disabled" {
		t.Fatal(status)
	}
}
func TestTabbyUpgradeAndRepair(t *testing.T) {
	integration := integrationFixture(t, true)
	if err := integration.Install(); err != nil {
		t.Fatal(err)
	}
	// Simulate an older managed release with its original content manifest.
	path := filepath.Join(integration.PluginPath(), "bridge.js")
	previous := []byte("previous release")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	manifest, _ := integration.manifest()
	manifest.Files["bridge.js"] = digest(previous)
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(integration.PluginPath(), tabbyMarker), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := integration.Configure("auto", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	status, _ := integration.Status()
	if !status.Current {
		t.Fatal("old release not updated")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := integration.Configure("auto", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	status, _ = integration.Status()
	if !status.Current {
		t.Fatal("missing file not repaired")
	}
}
func TestTabbyPreservesForeignAndModifiedFiles(t *testing.T) {
	integration := integrationFixture(t, true)
	if err := os.MkdirAll(integration.PluginPath(), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(integration.PluginPath(), "package.json")
	foreign := []byte(`{"name":"foreign"}`)
	if err := os.WriteFile(file, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if integration.Install() == nil {
		t.Fatal("overwrote foreign plugin")
	}
	if err := integration.Remove(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(file)
	if !bytes.Equal(got, foreign) {
		t.Fatal("removed foreign plugin")
	}
	if err := os.RemoveAll(integration.PluginPath()); err != nil {
		t.Fatal(err)
	}
	if err := integration.Install(); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(integration.PluginPath(), "index.js")
	custom := []byte("user change")
	if err := os.WriteFile(file, custom, 0600); err != nil {
		t.Fatal(err)
	}
	if integration.Install() == nil || integration.Remove() == nil {
		t.Fatal("modified plugin not protected")
	}
	got, _ = os.ReadFile(file)
	if !bytes.Equal(got, custom) {
		t.Fatal("modified file changed")
	}
}
func TestTabbyUninstallKeepsOtherPluginsAndExtraFiles(t *testing.T) {
	integration := integrationFixture(t, true)
	if err := integration.Install(); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(filepath.Dir(integration.PluginPath()), "tabby-other", "index.js")
	if err := os.MkdirAll(filepath.Dir(other), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(integration.PluginPath(), "notes.txt")
	if err := os.WriteFile(extra, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := integration.Remove(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{other, extra} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(integration.PluginPath(), "index.js")); !os.IsNotExist(err) {
		t.Fatal("managed plugin not removed")
	}
	if preference, _ := integration.preference(); preference != "" {
		t.Fatal("preference not removed")
	}
}
func TestTabbyRejectsSymlinkDirectories(t *testing.T) {
	integration := integrationFixture(t, true)
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(integration.PluginPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, integration.PluginPath()); err != nil {
		t.Fatal(err)
	}
	if integration.Install() == nil || integration.Remove() == nil {
		t.Fatal("followed plugin symlink")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("modified symlink target")
	}
}

func TestTabbyRejectsParentSymlinkOnUninstall(t *testing.T) {
	integration := integrationFixture(t, true)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(integration.HomeDir, "Library")); err != nil {
		t.Fatal(err)
	}
	if integration.Install() == nil || integration.Remove() == nil {
		t.Fatal("followed parent symlink")
	}
}
func TestTabbyAdoptsMatchingManualInstall(t *testing.T) {
	integration := integrationFixture(t, true)
	if err := integration.Install(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(integration.PluginPath(), tabbyMarker)); err != nil {
		t.Fatal(err)
	}
	if err := integration.Configure("auto", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	status, _ := integration.Status()
	if !status.Managed || !status.Current {
		t.Fatal(status)
	}
}

func TestTabbyUpgradeVerifiedPreviousContent(t *testing.T) {
	i := integrationFixture(t, true)
	if err := i.Install(); err != nil {
		t.Fatal(err)
	}
	m, err := i.manifest()
	if err != nil {
		t.Fatal(err)
	}
	old := []byte("previous managed bridge")
	if err := os.WriteFile(filepath.Join(i.PluginPath(), "bridge.js"), old, 0600); err != nil {
		t.Fatal(err)
	}
	m.Files["bridge.js"] = digest(old)
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(i.PluginPath(), tabbyMarker), data, 0600); err != nil {
		t.Fatal(err)
	}
	status, _ := i.Status()
	if status.Current {
		t.Fatal("old content reported current")
	}
	if err := i.Install(); err != nil {
		t.Fatal(err)
	}
	status, _ = i.Status()
	if !status.Current {
		t.Fatal("verified upgrade failed")
	}
}
