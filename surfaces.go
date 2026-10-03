package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/han1eng/agent-bell/internal/notify"
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/builtin"
)

// Discovery schema v1. Detected means part of this shell's stack; installed is separate.
type providerInfo struct {
	Maturity     string                     `json:"maturity"`
	Validation   string                     `json:"validation"`
	Name         string                     `json:"name"`
	Installed    bool                       `json:"installed"`
	Detected     bool                       `json:"detected"`
	Capabilities []surface.ReturnCapability `json:"capabilities"`
	Status       string                     `json:"status"`
}
type currentInfo struct {
	Outer      string                   `json:"outer"`
	Inner      []string                 `json:"inner"`
	Capability surface.ReturnCapability `json:"capability"`
	Layers     []layerInfo              `json:"layers"`
}
type layerInfo struct {
	Provider string              `json:"provider"`
	Probe    surface.ProbeResult `json:"probe"`
}
type discovery struct {
	SchemaVersion int            `json:"schema_version"`
	Current       currentInfo    `json:"current"`
	Providers     []providerInfo `json:"providers"`
}

func installedProvider(name string) bool {
	if name == "generic" {
		return true
	}
	if name == "tmux" {
		_, err := exec.LookPath("tmux")
		return err == nil
	}
	apps := map[string][]string{"tabby": {"Tabby"}, "terminal": {"Terminal"}, "iterm": {"iTerm", "iTerm2"}, "wezterm": {"WezTerm"}, "jetbrains": {"GoLand", "IntelliJ IDEA"}, "ghostty": {"Ghostty"}}
	home, _ := os.UserHomeDir()
	for _, root := range []string{"/Applications", "/System/Applications/Utilities", filepath.Join(home, "Applications")} {
		for _, app := range apps[name] {
			if _, err := os.Stat(filepath.Join(root, app+".app")); err == nil {
				return true
			}
		}
	}
	return false
}
func discover(r surface.Registry, t surface.ReturnTarget, installed func(string) bool, probes ...func(surface.SurfaceProvider, surface.ReturnTarget) surface.ProbeResult) discovery {
	d := discovery{SchemaVersion: 1, Current: currentInfo{Outer: t.Surface, Inner: []string{}, Capability: t.Capability, Layers: []layerInfo{}}, Providers: []providerInfo{}}
	layers := t.Layers
	if len(layers) == 0 {
		layers = []surface.SurfaceLayer{surface.Layer(t)}
	}
	for i, l := range layers {
		if i > 0 {
			d.Current.Inner = append(d.Current.Inner, l.Provider)
		}
		result := surface.ProbeResult{Status: "unsupported", Capability: l.Capability, Reason: surface.UnsupportedSurface}
		if p := r.Provider(l.Provider); p != nil && l.ContextID != "" {
			attempt := l.Target(t)
			attempt.Capability = surface.ReturnExactContext
			if len(probes) > 0 {
				result = probes[0](p, attempt)
			} else {
				result = surface.Probe(p, attempt)
			}
		}
		d.Current.Layers = append(d.Current.Layers, layerInfo{l.Provider, result})
	}
	if t.Capability == surface.ReturnExactContext {
		for _, l := range d.Current.Layers {
			if !l.Probe.Valid {
				d.Current.Capability = surface.ReturnApp
				if t.AppBundleID == "" {
					d.Current.Capability = surface.ReturnProject
				}
				break
			}
		}
	}
	for _, p := range r.Providers() {
		detected := false
		status := "not_current"
		for _, l := range d.Current.Layers {
			if l.Provider == p.Name() || p.Name() == "generic" && r.Provider(l.Provider) == nil {
				detected = true
				status = l.Probe.Status
			}
		}
		d.Providers = append(d.Providers, providerInfo{Maturity: builtin.Maturity(p.Name()), Validation: builtin.Validation(p.Name()), Name: p.Name(), Installed: installed(p.Name()), Detected: detected, Capabilities: surface.Capabilities(p), Status: status})
	}
	return d
}
func surfacesCommand(args []string, w io.Writer) error {
	if len(args) > 1 || len(args) == 1 && args[0] != "--json" {
		return errors.New("usage: agentbell surfaces [--json]")
	}
	cwd, _ := os.Getwd()
	r := builtin.Registry("")
	d := discover(r, *r.Detect(surface.DetectContext{CWD: cwd}), installedProvider, nonPromptingProbe)
	if len(args) == 1 {
		return json.NewEncoder(w).Encode(d)
	}
	fmt.Fprint(w, "AgentBell Surfaces\n\n")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SURFACE\tMATURITY\tINSTALLED\tDETECTED\tAPP\tWINDOW\tEXACT")
	for _, p := range d.Providers {
		flags := map[surface.ReturnCapability]string{}
		for _, c := range p.Capabilities {
			flags[c] = "✓"
		}
		value := func(c surface.ReturnCapability) string {
			if flags[c] != "" {
				return flags[c]
			}
			return "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%t\t%t\t%s\t%s\t%s\n", p.Name, p.Maturity, p.Installed, p.Detected, value(surface.ReturnApp), value(surface.ReturnWindow), value(surface.ReturnExactContext))
	}
	tw.Flush()
	fmt.Fprintf(w, "\nCurrent Context\nOuter Surface: %s\nMultiplexer: %s\nBest Capability: %s\n", d.Current.Outer, strings.Join(d.Current.Inner, ", "), d.Current.Capability)
	for _, l := range d.Current.Layers {
		fmt.Fprintf(w, "%s: %s", l.Provider, l.Probe.Status)
		if l.Probe.Reason != "" {
			fmt.Fprintf(w, " (%s)", l.Probe.Reason)
		}
		fmt.Fprintln(w)
	}
	return nil
}
func printReturnStack(w io.Writer, t surface.ReturnTarget) {
	d := discover(builtin.Registry(""), t, installedProvider, nonPromptingProbe)
	fmt.Fprintln(w, "\nReturn Stack")
	for i, l := range d.Current.Layers {
		kind := "Outer Surface"
		if i > 0 {
			kind = "Inner Surface"
		}
		fmt.Fprintf(w, "%s: %s\n  Context: %s", kind, l.Provider, l.Probe.Status)
		if l.Probe.Reason != "" {
			fmt.Fprintf(w, " (%s)", l.Probe.Reason)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Best Return: %s\n", d.Current.Capability)
	fmt.Fprintln(w, "\nSurface Integrations")
	for _, p := range d.Providers {
		if p.Name != "generic" {
			fmt.Fprintf(w, "%s: maturity=%s installed=%t current=%t status=%s\n", p.Name, p.Maturity, p.Installed, p.Detected, p.Status)
		}
	}
}

// Diagnostics must not request Automation access. Click-time Focus may request
// normal macOS consent; discovery remains a read-only health operation.
func nonPromptingProbe(p surface.SurfaceProvider, t surface.ReturnTarget) surface.ProbeResult {
	if p.Name() == "terminal" || p.Name() == "iterm" {
		result := surface.ProbeResult{Status: "unreachable", Capability: t.Capability, Reason: surface.ProviderUnavailable}
		executable, _ := os.Executable()
		helper := notify.NativeHelperFor(executable)
		if helper == "" {
			return result
		}
		flag := "--check-terminal-automation"
		if p.Name() == "iterm" {
			flag = "--check-iterm-automation"
		}
		out, err := exec.Command(helper, flag).CombinedOutput()
		if err != nil {
			return result
		}
		switch strings.TrimSpace(string(out)) {
		case "authorized":
			return surface.Probe(p, t)
		case "denied", "not_requested":
			result.Reason = surface.PermissionDenied
		case "app_not_running":
			result.Reason = surface.AppNotRunning
		}
		return result
	}
	return surface.Probe(p, t)
}
