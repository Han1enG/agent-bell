package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/jetbrains"
	"github.com/han1eng/agent-bell/internal/surface/tabby"
)

func printCurrentContext(w io.Writer, target surface.ReturnTarget, env func(string) string, probe func(surface.ReturnTarget) error) {
	if len(target.Layers) > 0 {
		return
	}
	fmt.Fprintln(w, "\nCurrent Session")
	name := target.Surface
	if name == "generic" || name == "" {
		name = "unknown"
	}
	fmt.Fprintf(w, "Surface: %s\n", name)
	if target.AppBundleID != "" {
		fmt.Fprintf(w, "✓ App return available: %s (%s); activation not exercised\n", target.AppName, target.AppBundleID)
	} else {
		fmt.Fprintln(w, "○ Origin App unavailable")
	}
	if target.Surface == "tabby" || target.Surface == "jetbrains" {
		if env("AGENTBELL_SURFACE") != target.Surface || env("AGENTBELL_CONTEXT_ID") == "" {
			fmt.Fprintln(w, "○ AGENTBELL_SURFACE / AGENTBELL_CONTEXT_ID not found for this session\n○ Exact session return unavailable in this shell.\nFix: Restart the originating app and open a new local terminal tab.")
			return
		}
	}
	if target.ContextID == "" {
		fmt.Fprintln(w, "○ Exact session return unavailable: no context identity; this is not an installation failure.")
		return
	}
	attempt := target
	attempt.Capability = surface.ReturnExactContext
	if err := probe(attempt); err != nil {
		fmt.Fprintf(w, "○ Exact session return unavailable: reason=%s\n", surface.Reason(err))
		if target.Surface == "terminal" && surface.Reason(err) == surface.PermissionDenied {
			fmt.Fprintln(w, "✗ Terminal Automation permission denied; App return remains available.")
		}
		return
	}
	fmt.Fprintf(w, "✓ Context ID: %s\n✓ Exact session return available (live read-only probe)\n", target.ContextID)
}

func probeProvider(target surface.ReturnTarget, providers []surface.SurfaceProvider) error {
	for _, provider := range providers {
		if provider.CanHandle(target) {
			if probe, ok := provider.(surface.ProbeableProvider); ok {
				return probe.Probe(target)
			}
		}
	}
	return surface.Fail(surface.UnsupportedSurface, errors.New("no read-only provider"))
}

// Integration reachability and shell identity are deliberately separate.
func printBridgeHealth(w io.Writer, home, name string) {
	paths, err := filepath.Glob(filepath.Join(home, ".cache", "agentbell", name, "*.sock"))
	reachable := 0
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".sock")
		if name == "tabby" {
			_, err = (tabby.Provider{}).List(id)
		} else {
			_, err = (jetbrains.Provider{}).List(id)
		}
		if err == nil {
			reachable++
		}
	}
	if reachable > 0 {
		fmt.Fprintf(w, "✓ Plugin bridge reachable (%d instance(s)); does not establish this shell's context\n", reachable)
	} else {
		fmt.Fprintln(w, "○ Plugin bridge not reachable; restart the app after installation")
	}
}

func currentEnv(key string) string { return os.Getenv(key) }
