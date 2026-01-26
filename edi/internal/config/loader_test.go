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

	if !cfg.Briefing.IncludeHistory {
		t.Error("Expected briefing to include history by default")
	}

	if cfg.Briefing.HistoryEntries != 3 {
		t.Errorf("Expected 3 history entries, got %d", cfg.Briefing.HistoryEntries)
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
