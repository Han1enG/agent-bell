package attention

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Only explicit title metadata is used, never conversation/prompt text.
func CleanTitle(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") || utf8.RuneCountInString(value) > 160 {
		return ""
	}
	return value
}
func LookupTitle(home, source, sessionID, cwd string) string {
	if sessionID == "" || len(sessionID) > 512 {
		return ""
	}
	if source == "codex" {
		return codexTitle(home, sessionID)
	}
	if source != "claude" {
		return ""
	}
	// Claude's optional session index contains summaries/custom titles only.
	folder := strings.NewReplacer("/", "-", "\\", "-", ".", "-", "_", "-").Replace(cwd)
	path := filepath.Join(home, ".claude", "projects", folder, "sessions-index.json")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var index struct {
		Entries []struct {
			SessionID   string `json:"sessionId"`
			CustomTitle string `json:"customTitle"`
			Summary     string `json:"summary"`
		} `json:"entries"`
	}
	if json.Unmarshal(data, &index) != nil {
		return ""
	}
	for _, entry := range index.Entries {
		if entry.SessionID == sessionID {
			if title := CleanTitle(entry.CustomTitle); title != "" {
				return title
			}
			return CleanTitle(entry.Summary)
		}
	}
	return ""
}
