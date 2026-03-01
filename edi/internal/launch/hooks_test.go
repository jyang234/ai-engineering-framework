package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

func TestUpdateHooksSettings_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = true

	if err := UpdateHooksSettings(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("missing hooks key")
	}

	// Verify all four events exist
	for _, event := range []string{"PreToolUse", "PostToolUse", "PostToolUseFailure", "PreCompact"} {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing event %s", event)
		}
	}
}

func TestUpdateHooksSettings_GuardDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = false

	if err := UpdateHooksSettings(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should not be created when guard is disabled
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("file should not exist when guard is disabled")
	}
}

func TestUpdateHooksSettings_PreservesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	// Write existing settings with a custom hook
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "my-custom-hook",
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), data, 0644)

	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = true

	if err := UpdateHooksSettings(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back
	data, _ = os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooks := settings["hooks"].(map[string]interface{})
	preToolUse := hooks["PreToolUse"].([]interface{})

	// Should have the original Bash matcher group
	bashGroup := preToolUse[0].(map[string]interface{})
	handlers := bashGroup["hooks"].([]interface{})

	// Should have both the custom hook AND edi-guard
	if len(handlers) != 2 {
		t.Fatalf("expected 2 handlers, got %d", len(handlers))
	}

	// Verify custom hook preserved
	h0 := handlers[0].(map[string]interface{})
	if h0["command"] != "my-custom-hook" {
		t.Errorf("custom hook not preserved: %v", h0["command"])
	}

	// Verify edi-guard added
	h1 := handlers[1].(map[string]interface{})
	if h1["command"] != ediGuardCommand {
		t.Errorf("edi-guard not added: %v", h1["command"])
	}
}

func TestUpdateHooksSettings_Idempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = true

	// Run twice
	UpdateHooksSettings(dir, cfg)
	UpdateHooksSettings(dir, cfg)

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooks := settings["hooks"].(map[string]interface{})
	preToolUse := hooks["PreToolUse"].([]interface{})

	// Should only have one Bash matcher group
	bashGroup := preToolUse[0].(map[string]interface{})
	handlers := bashGroup["hooks"].([]interface{})

	// Should have exactly one edi-guard handler, not duplicates
	ediGuardCount := 0
	for _, h := range handlers {
		handler := h.(map[string]interface{})
		cmd, _ := handler["command"].(string)
		if cmd == ediGuardCommand {
			ediGuardCount++
		}
	}
	if ediGuardCount != 1 {
		t.Fatalf("expected 1 edi-guard handler, got %d", ediGuardCount)
	}
}

func TestUpdateHooksSettings_PreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	// Write existing settings with non-hook keys
	existing := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []interface{}{"Read", "Write"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), data, 0644)

	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = true

	UpdateHooksSettings(dir, cfg)

	data, _ = os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	// Permissions should still be there
	if _, ok := settings["permissions"]; !ok {
		t.Error("existing permissions key was lost")
	}
	// Hooks should be added
	if _, ok := settings["hooks"]; !ok {
		t.Error("hooks key was not added")
	}
}

func TestMergeHookEntry_DifferentMatchers(t *testing.T) {
	hooks := make(map[string]interface{})

	// Add Bash matcher
	mergeHookEntry(hooks, "PreToolUse", "Bash")
	// Add .* matcher (different matcher for same event)
	mergeHookEntry(hooks, "PreToolUse", ".*")

	groups := hooks["PreToolUse"].([]interface{})
	if len(groups) != 2 {
		t.Fatalf("expected 2 matcher groups, got %d", len(groups))
	}
}

func TestReadJSONFile_NotFound(t *testing.T) {
	result, err := readJSONFile("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("should return empty map, got %v", result)
	}
}

func TestReadJSONFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := readJSONFile(path)
	if err == nil {
		t.Fatal("should error on invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Guard disable cleanup tests
// ---------------------------------------------------------------------------

func TestUpdateHooksSettings_DisableRemovesEntries(t *testing.T) {
	dir := t.TempDir()

	// First, enable guard to create the entries
	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = true
	UpdateHooksSettings(dir, cfg)

	// Verify entries exist
	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]interface{})
	if len(hooks) == 0 {
		t.Fatal("hooks should exist after enabling")
	}

	// Now disable guard
	cfg.Guard.Enabled = false
	if err := UpdateHooksSettings(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify edi-guard entries are removed
	data, _ = os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	json.Unmarshal(data, &settings)
	hooks, ok := settings["hooks"].(map[string]interface{})
	if ok && len(hooks) > 0 {
		// Check no edi-guard handlers remain
		for event, val := range hooks {
			groups, ok := val.([]interface{})
			if !ok {
				continue
			}
			for _, g := range groups {
				group, ok := g.(map[string]interface{})
				if !ok {
					continue
				}
				handlers, _ := group["hooks"].([]interface{})
				for _, h := range handlers {
					handler, ok := h.(map[string]interface{})
					if !ok {
						continue
					}
					cmd, _ := handler["command"].(string)
					if cmd == ediGuardCommand {
						t.Errorf("edi-guard handler still present in %s", event)
					}
				}
			}
		}
	}
}

func TestUpdateHooksSettings_DisablePreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	// Write settings with both custom and edi-guard hooks
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "my-custom-hook",
						},
						map[string]interface{}{
							"type":    "command",
							"command": ediGuardCommand,
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), data, 0644)

	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = false
	UpdateHooksSettings(dir, cfg)

	// Read back
	data, _ = os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooks := settings["hooks"].(map[string]interface{})
	preToolUse := hooks["PreToolUse"].([]interface{})
	bashGroup := preToolUse[0].(map[string]interface{})
	handlers := bashGroup["hooks"].([]interface{})

	// Should have only the custom hook
	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}
	h0 := handlers[0].(map[string]interface{})
	if h0["command"] != "my-custom-hook" {
		t.Errorf("custom hook not preserved: %v", h0["command"])
	}
}

func TestUpdateHooksSettings_DisableNoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Guard.Enabled = false

	// Should not error or create file
	if err := UpdateHooksSettings(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("should not create file when disabling with no existing file")
	}
}

func TestEnsureEdiGuardHandler_ExactMatch(t *testing.T) {
	group := map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "/path/to/my-edi-guard-wrapper",
			},
		},
	}

	ensureEdiGuardHandler(group)

	handlers := group["hooks"].([]interface{})
	// Should add edi-guard as a new handler, not overwrite the wrapper
	if len(handlers) != 2 {
		t.Fatalf("expected 2 handlers (wrapper + edi-guard), got %d", len(handlers))
	}
	h0 := handlers[0].(map[string]interface{})
	if h0["command"] != "/path/to/my-edi-guard-wrapper" {
		t.Error("wrapper should not be modified")
	}
	h1 := handlers[1].(map[string]interface{})
	if h1["command"] != ediGuardCommand {
		t.Error("edi-guard should be appended")
	}
}
