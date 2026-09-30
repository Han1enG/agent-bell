package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/han1eng/agent-bell/internal/notify"
)

const marker = "notify --source"

type Installer struct {
	HomeDir    string
	Executable string
}

func New(homeDir, executable string) Installer {
	return Installer{HomeDir: homeDir, Executable: executable}
}

func (i Installer) Install() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("AgentBell v0.1 supports macOS only")
	}
	if i.HomeDir == "" || i.Executable == "" {
		return fmt.Errorf("home directory and executable are required")
	}
	if notify.NativeHelperFor(i.Executable) == "" {
		return fmt.Errorf("native notification helper is missing; install the packaged AgentBell.app first")
	}
	command := shellQuote(i.Executable) + " notify --source"
	if err := i.updateJSON(filepath.Join(i.HomeDir, ".claude", "settings.json"), func(root map[string]any) {
		addClaudeHook(root, "Notification", "agent_completed|agent_needs_input", command+" claude")
		addClaudeHook(root, "PermissionRequest", "", command+" claude")
		addClaudeHook(root, "Stop", "", command+" claude")
		addClaudeHook(root, "StopFailure", "", command+" claude")
	}); err != nil {
		return fmt.Errorf("install Claude Code hook: %w", err)
	}
	if err := i.updateJSON(filepath.Join(i.HomeDir, ".codex", "hooks.json"), func(root map[string]any) {
		// Rebuild only AgentBell entries so old experimental mappings are removed
		// without touching hooks owned by the user or another tool.
		removeHooks(root)
		for _, name := range []string{"Stop", "PermissionRequest"} {
			addCodexHook(root, name, command+" codex")
		}
	}); err != nil {
		return fmt.Errorf("install Codex hook: %w", err)
	}
	return nil
}

func (i Installer) Uninstall() error {
	if err := i.updateJSONIfExists(filepath.Join(i.HomeDir, ".claude", "settings.json"), removeClaudeHooks); err != nil {
		return fmt.Errorf("uninstall Claude Code hook: %w", err)
	}
	if err := i.updateJSONIfExists(filepath.Join(i.HomeDir, ".codex", "hooks.json"), removeCodexHooks); err != nil {
		return fmt.Errorf("uninstall Codex hook: %w", err)
	}
	if appPath := nativeAppPath(i.HomeDir, i.Executable); appPath != "" {
		if err := os.RemoveAll(appPath); err != nil {
			return fmt.Errorf("remove AgentBell.app: %w", err)
		}
	}
	launcher := filepath.Join(i.HomeDir, "bin", "agentbell")
	if target, err := filepath.EvalSymlinks(launcher); err == nil && strings.Contains(target, "AgentBell.app/Contents/MacOS/agentbell") {
		if err := os.Remove(launcher); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove AgentBell launcher: %w", err)
		}
	}
	return nil
}

func nativeAppPath(homeDir, executable string) string {
	marker := string(filepath.Separator) + "AgentBell.app" + string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS" + string(filepath.Separator)
	if index := strings.Index(executable, marker); index >= 0 {
		return executable[:index+len(string(filepath.Separator))+len("AgentBell.app")]
	}
	path := filepath.Join(homeDir, "Applications", "AgentBell.app")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func HasAgentBellHooks(path string, requiredEvents []string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, "config file missing", nil
	}
	if err != nil {
		return false, "", err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, "invalid JSON", err
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return false, "hooks missing", nil
	}
	for _, eventName := range requiredEvents {
		if !eventHasAgentBell(hooks[eventName]) {
			return false, eventName, nil
		}
	}
	return true, "", nil
}

func eventHasAgentBell(value any) bool {
	groups, ok := value.([]any)
	if !ok {
		return false
	}
	for _, groupValue := range groups {
		group, ok := groupValue.(map[string]any)
		if !ok {
			continue
		}
		for _, handler := range array(group, "hooks") {
			if isAgentBell(handler) {
				return true
			}
		}
	}
	return false
}

func addClaudeHook(root map[string]any, eventName, matcher, command string) {
	hooks := object(root, "hooks")
	groups := array(hooks, eventName)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if handlers, ok := group["hooks"].([]any); ok {
			for _, handler := range handlers {
				if isAgentBell(handler) {
					if matcher != "" {
						group["matcher"] = matcher
					}
					return
				}
			}
		}
	}
	group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "async": true}}}
	if matcher != "" {
		group["matcher"] = matcher
	}
	hooks[eventName] = append(groups, group)
}

func addCodexHook(root map[string]any, eventName, command string) {
	hooks := object(root, "hooks")
	groups := array(hooks, eventName)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, handler := range array(group, "hooks") {
			if isAgentBell(handler) {
				return
			}
		}
	}
	hooks[eventName] = append(groups, map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "async": true}}})
}

func removeClaudeHooks(root map[string]any) { removeHooks(root) }
func removeCodexHooks(root map[string]any)  { removeHooks(root) }

func removeHooks(root map[string]any) {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return
	}
	for eventName, value := range hooks {
		groups, ok := value.([]any)
		if !ok {
			continue
		}
		keptGroups := make([]any, 0, len(groups))
		for _, item := range groups {
			group, ok := item.(map[string]any)
			if !ok {
				keptGroups = append(keptGroups, item)
				continue
			}
			handlers, ok := group["hooks"].([]any)
			if !ok {
				keptGroups = append(keptGroups, item)
				continue
			}
			kept := make([]any, 0, len(handlers))
			for _, handler := range handlers {
				if !isAgentBell(handler) {
					kept = append(kept, handler)
				}
			}
			if len(kept) > 0 {
				group["hooks"] = kept
				keptGroups = append(keptGroups, group)
			}
		}
		if len(keptGroups) > 0 {
			hooks[eventName] = keptGroups
		} else {
			delete(hooks, eventName)
		}
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	}
}

func isAgentBell(value any) bool {
	handler, ok := value.(map[string]any)
	if !ok {
		return false
	}
	command, _ := handler["command"].(string)
	return strings.Contains(command, marker)
}

func object(root map[string]any, key string) map[string]any {
	if value, ok := root[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	root[key] = value
	return value
}

func array(root map[string]any, key string) []any {
	if value, ok := root[key].([]any); ok {
		return value
	}
	return nil
}

func (i Installer) updateJSON(path string, update func(map[string]any)) error {
	return i.updateJSONWith(path, update, false)
}

func (i Installer) updateJSONIfExists(path string, update func(map[string]any)) error {
	return i.updateJSONWith(path, update, true)
}

func (i Installer) updateJSONWith(path string, update func(map[string]any), skipMissing bool) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && skipMissing {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	root := map[string]any{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	update(root)
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agentbell-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(encoded); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
