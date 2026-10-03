package builtin

import (
	"strings"

	"github.com/han1eng/agent-bell/internal/surface"
)

// Maturity describes validation, independently of runtime capability.
func Maturity(name string) string {
	switch name {
	case "iterm", "wezterm":
		return "experimental"
	default:
		return "stable"
	}
}
func Validation(name string) string {
	switch name {
	case "iterm", "wezterm":
		return "fixture only; real GUI pending"
	case "jetbrains":
		return "GoLand 2025.3 / build 253; bridge regression"
	case "tmux":
		return "real tmux CLI; composite GUI pending"
	default:
		return "regression tested; v0.3 GUI closure pending"
	}
}

func CapabilityMatrix() string {
	var b strings.Builder
	b.WriteString("| Provider | Maturity | App | Window | Exact | Project | Validation |\n|---|---|---|---|---|---|---|\n")
	for _, p := range Registry("").Providers() {
		b.WriteString("| " + p.Name() + " | " + Maturity(p.Name()) + " |")
		caps := map[surface.ReturnCapability]bool{}
		for _, c := range surface.Capabilities(p) {
			caps[c] = true
		}
		for _, c := range []surface.ReturnCapability{surface.ReturnApp, surface.ReturnWindow, surface.ReturnExactContext, surface.ReturnProject} {
			if caps[c] {
				b.WriteString(" ✓ |")
			} else {
				b.WriteString(" — |")
			}
		}
		b.WriteString(" " + Validation(p.Name()) + " |\n")
	}
	return b.String()
}
