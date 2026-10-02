package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Notifications struct {
	Done, NeedsInput, NeedsApproval, Error bool
}

type Terminal struct{ App string }

type Return struct {
	Enabled     bool
	FallbackApp string
}

type Config struct {
	Return        Return
	Notifications Notifications
	Terminal      Terminal
}

func Defaults() Config {
	return Config{Notifications: Notifications{true, true, true, true}, Terminal: Terminal{App: "terminal"}, Return: Return{Enabled: true, FallbackApp: "auto"}}
}

func Path(home string) string { return filepath.Join(home, ".config", "agentbell", "config.toml") }

// Load reads the deliberately small TOML subset supported by AgentBell.
func Load(path string) (Config, error) {
	c := Defaults()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	section := ""
	for line := 1; s.Scan(); line++ {
		text := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
			section = strings.TrimSpace(text[1 : len(text)-1])
			continue
		}
		parts := strings.SplitN(text, "=", 2)
		if len(parts) != 2 {
			return c, fmt.Errorf("line %d: expected key = value", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if section == "notifications" {
			b, e := strconv.ParseBool(value)
			if e != nil {
				return c, fmt.Errorf("line %d: %s must be true or false", line, key)
			}
			switch key {
			case "done":
				c.Notifications.Done = b
			case "needs_input":
				c.Notifications.NeedsInput = b
			case "needs_approval":
				c.Notifications.NeedsApproval = b
			case "error":
				c.Notifications.Error = b
			default:
				return c, fmt.Errorf("line %d: unknown notifications key %q", line, key)
			}
		} else if section == "return" {
			switch key {
			case "enabled":
				b, err := strconv.ParseBool(value)
				if err != nil {
					return c, fmt.Errorf("line %d: return.enabled must be boolean", line)
				}
				c.Return.Enabled = b
			case "fallback_app":
				v, err := strconv.Unquote(value)
				if err != nil || strings.TrimSpace(v) == "" {
					return c, fmt.Errorf("line %d: fallback_app must be a nonempty quoted string", line)
				}
				c.Return.FallbackApp = v
			default:
				return c, fmt.Errorf("line %d: unknown return key %q", line, key)
			}
		} else if section == "terminal" {
			v, e := strconv.Unquote(value)
			if e != nil {
				return c, fmt.Errorf("line %d: terminal.app must be a quoted string", line)
			}
			if key != "app" {
				return c, fmt.Errorf("line %d: unknown terminal key %q", line, key)
			}
			c.Terminal.App = strings.ToLower(v)
			c.Return.FallbackApp = c.Terminal.App
		} else {
			return c, fmt.Errorf("line %d: unsupported section %q", line, section)
		}
	}
	if err := s.Err(); err != nil {
		return c, err
	}
	if c.Terminal.App != "terminal" && c.Terminal.App != "iterm2" {
		return c, fmt.Errorf("terminal.app must be terminal or iterm2")
	}
	return c, nil
}

func (c Config) Allows(t string) bool {
	switch t {
	case "done":
		return c.Notifications.Done
	case "needs_input":
		return c.Notifications.NeedsInput
	case "needs_approval":
		return c.Notifications.NeedsApproval
	case "error":
		return c.Notifications.Error
	}
	return false
}
