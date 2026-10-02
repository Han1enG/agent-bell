package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func golandFixture(t *testing.T) GoLandIntegration {
	t.Helper()
	home := t.TempDir()
	app := filepath.Join(home, "Applications", "GoLand.app")
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "product-info.json"), []byte(`{"version":"2025.3.5.1","buildNumber":"253.33813.70","dataDirectoryName":"GoLand2025.3","productCode":"GO"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return GoLandIntegration{HomeDir: home, AppPaths: []string{app}}
}
func TestGoLandInstallPreferenceAndRemoval(t *testing.T) {
	g := golandFixture(t)
	var output bytes.Buffer
	if err := g.Configure("skip", &output); err != nil {
		t.Fatal(err)
	}
	if err := g.Configure("auto", &output); err != nil {
		t.Fatal(err)
	}
	s, _ := g.Status()
	if s.Installed {
		t.Fatal("ignored opt-out")
	}
	if err := g.Configure("enable", &output); err != nil {
		t.Fatal(err)
	}
	s, err := g.Status()
	if err != nil || !s.Installed || !s.Managed || !s.Current || s.Preference != "enabled" {
		t.Fatal(s, err)
	}
	if err := g.Install(); err != nil {
		t.Fatal("repeat install", err)
	}
	extra := filepath.Join(s.Path, "lib", "user.txt")
	if err := os.WriteFile(extra, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.Install(); err == nil {
		t.Fatal("replaced unexpected files")
	}
	if err := g.Remove(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(extra); err != nil || string(data) != "keep" {
		t.Fatal("removed extra file", err)
	}
}
func TestGoLandProtectsModifiedForeignAndLinkedFiles(t *testing.T) {
	for _, mode := range []string{"modified", "foreign", "link"} {
		t.Run(mode, func(t *testing.T) {
			g := golandFixture(t)
			if err := g.Install(); err != nil {
				t.Fatal(err)
			}
			s, _ := g.Status()
			path := filepath.Join(s.Path, golandJar)
			switch mode {
			case "modified":
				_ = os.WriteFile(path, []byte("custom"), 0600)
			case "foreign":
				_ = os.Remove(filepath.Join(s.Path, tabbyMarker))
			case "link":
				_ = os.Remove(path)
				_ = os.Symlink(filepath.Join(s.Path, tabbyMarker), path)
			}
			if err := g.Install(); err == nil {
				t.Fatal("overwrote foreign/edited file")
			}
			if mode != "foreign" {
				if err := g.Remove(); err == nil {
					t.Fatal("removed edited file")
				}
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal("file lost", err)
			}
		})
	}
}
func TestGoLandRejectsUnsupportedAndUnsafePaths(t *testing.T) {
	for _, product := range []string{
		`{"productCode":"GO","buildNumber":"261.1","dataDirectoryName":"GoLand2026.1"}`,
		`{"productCode":"GO","buildNumber":"253.1","dataDirectoryName":"../../escape"}`,
	} {
		g := golandFixture(t)
		_ = os.WriteFile(filepath.Join(g.AppPaths[0], "Contents", "Resources", "product-info.json"), []byte(product), 0600)
		if err := g.Install(); err == nil {
			t.Fatal("accepted unsupported/unsafe IDE")
		}
	}
	g := golandFixture(t)
	root := filepath.Join(g.HomeDir, "Library")
	if err := os.Symlink(t.TempDir(), root); err != nil {
		t.Fatal(err)
	}
	if err := g.Install(); err == nil {
		t.Fatal("followed symlink parent")
	}
}
