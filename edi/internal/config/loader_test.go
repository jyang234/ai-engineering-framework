package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", cfg.Version)
	}

	if cfg.Agent != "coder" {
		t.Errorf("Expected agent 'coder', got '%s'", cfg.Agent)
	}

	if !cfg.Recall.Enabled {
		t.Error("Expected recall to be enabled by default")
	}

	if cfg.Recall.Backend != "v0" {
		t.Errorf("Expected recall backend 'v0', got '%s'", cfg.Recall.Backend)
	}

	if !cfg.Briefing.IncludeHistory {
		t.Error("Expected briefing to include history by default")
	}

	if cfg.Briefing.HistoryEntries != 3 {
		t.Errorf("Expected 3 history entries, got %d", cfg.Briefing.HistoryEntries)
	}
}

func TestDefaultConfigCodexDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Codex.ModelsPath != "" {
		t.Errorf("Expected empty ModelsPath, got '%s'", cfg.Codex.ModelsPath)
	}

	if cfg.Codex.MetadataDB != "" {
		t.Errorf("Expected empty MetadataDB, got '%s'", cfg.Codex.MetadataDB)
	}
}

func TestWriteDefault(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "config.yaml")
	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Check for key content
	if len(content) < 100 {
		t.Error("Config file seems too small")
	}
}

func TestWriteProjectDefault(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "config.yaml")
	if err := WriteProjectDefault(path); err != nil {
		t.Fatalf("WriteProjectDefault failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}
}

func TestWriteDefaultWithBackend_V0(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "config.yaml")
	if err := WriteDefaultWithBackend(path, "v0"); err != nil {
		t.Fatalf("WriteDefaultWithBackend failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "backend: v0") {
		t.Error("Expected 'backend: v0' in config")
	}

	// Codex should be commented out for v0
	if contains(contentStr, "codex:\n  models_path:") {
		t.Error("Expected codex config to be commented out for v0 backend")
	}
}

func TestWriteDefaultWithBackend_Codex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "config.yaml")
	if err := WriteDefaultWithBackend(path, "codex"); err != nil {
		t.Fatalf("WriteDefaultWithBackend failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "backend: codex") {
		t.Error("Expected 'backend: codex' in config")
	}

	// Codex should be uncommented for codex backend
	if !contains(contentStr, "codex:\n  models_path:") {
		t.Error("Expected codex config to be uncommented for codex backend")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
