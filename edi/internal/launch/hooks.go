package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/aef/edi/internal/config"
)

// ediGuardCommand is the command path used to identify edi-guard hook entries.
const ediGuardCommand = "~/.edi/bin/edi-guard"

// hookEntry defines a single hook event registration for edi-guard.
type hookEntry struct {
	Event   string
	Matcher string
}

// ediGuardHooks lists the four hook events edi-guard registers for.
var ediGuardHooks = []hookEntry{
	{Event: "PreToolUse", Matcher: "Bash"},
	{Event: "PostToolUse", Matcher: "Bash"},
	{Event: "PostToolUseFailure", Matcher: "Bash"},
	{Event: "PreCompact", Matcher: ".*"},
}

// UpdateHooksSettings merges edi-guard hook entries into .claude/settings.json,
// preserving any existing non-edi hooks. Follows the same read-merge-write
// pattern as UpdateMCPConfig for .mcp.json.
func UpdateHooksSettings(projectDir string, cfg *config.Config) error {
	if !cfg.Guard.Enabled {
		return nil
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	// Read existing settings
	settings, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	// Get or create hooks map
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	// Merge each edi-guard hook entry
	for _, h := range ediGuardHooks {
		mergeHookEntry(hooks, h.Event, h.Matcher)
	}

	settings["hooks"] = hooks

	// Write back
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0644)
}

// mergeHookEntry ensures the hooks map has an edi-guard entry for the given event
// and matcher, without duplicating or removing other hooks.
func mergeHookEntry(hooks map[string]interface{}, event, matcher string) {
	// Get existing matcher groups for this event
	var groups []interface{}
	if existing, ok := hooks[event]; ok {
		if arr, ok := existing.([]interface{}); ok {
			groups = arr
		}
	}

	// Look for an existing matcher group with our matcher
	found := false
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		m, _ := group["matcher"].(string)
		if m != matcher {
			continue
		}
		// Found our matcher group — ensure edi-guard handler exists
		ensureEdiGuardHandler(group)
		found = true
		break
	}

	if !found {
		// Create new matcher group with edi-guard handler
		newGroup := map[string]interface{}{
			"matcher": matcher,
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": ediGuardCommand,
				},
			},
		}
		groups = append(groups, newGroup)
	}

	hooks[event] = groups
}

// ensureEdiGuardHandler ensures the matcher group has exactly one edi-guard handler.
func ensureEdiGuardHandler(group map[string]interface{}) {
	handlers, _ := group["hooks"].([]interface{})

	// Check if edi-guard handler already exists
	for _, h := range handlers {
		handler, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		cmd, _ := handler["command"].(string)
		if strings.Contains(cmd, "edi-guard") {
			handler["command"] = ediGuardCommand
			return
		}
	}

	// Not found — append
	handlers = append(handlers, map[string]interface{}{
		"type":    "command",
		"command": ediGuardCommand,
	})
	group["hooks"] = handlers
}

// readJSONFile reads a JSON file into a generic map. Returns empty map if
// the file doesn't exist.
func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if result == nil {
		result = make(map[string]interface{})
	}
	return result, nil
}
