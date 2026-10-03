// Package builtin owns the trusted, compiled-in provider registration order.
package builtin

import (
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/iterm"
	"github.com/han1eng/agent-bell/internal/surface/jetbrains"
	"github.com/han1eng/agent-bell/internal/surface/tabby"
	"github.com/han1eng/agent-bell/internal/surface/terminal"
	"github.com/han1eng/agent-bell/internal/surface/tmux"
	"github.com/han1eng/agent-bell/internal/surface/wezterm"
)

func Registry(fallback string) surface.Registry {
	var r surface.Registry
	r.Register(tmux.Provider{}, 100, true)
	r.Register(jetbrains.Provider{}, 90, false)
	r.Register(tabby.Provider{}, 90, false)
	r.Register(iterm.Provider{}, 70, false)
	r.Register(wezterm.Provider{}, 70, false)
	r.Register(terminal.Provider{}, 60, false)
	r.Register(surface.GenericProvider{FallbackApp: fallback}, -100, false)
	return r
}
