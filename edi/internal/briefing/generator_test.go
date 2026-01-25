package briefing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/edi/internal/config"
)

func TestGenerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "briefing-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .edi directory structure
	ediDir := filepath.Join(tmpDir, ".edi")
	os.MkdirAll(ediDir, 0755)
	os.MkdirAll(filepath.Join(ediDir, "history"), 0755)

	// Create a profile
	profile := `# Test Project

A test project for unit testing.
`
	os.WriteFile(filepath.Join(ediDir, "profile.md"), []byte(profile), 0644)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	cfg := config.DefaultConfig()
	brief, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if brief == "" {
		t.Error("Expected non-empty briefing")
	}

	// Should include profile content
	if !containsSubstring(brief, "Project Context") {
		t.Error("Expected briefing to include profile section")
	}
}

func TestLoadRecentHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create history directory
	historyDir := filepath.Join(tmpDir, ".edi", "history")
	os.MkdirAll(historyDir, 0755)

	// Create a history entry
	historyContent := `---
session_id: test-session-123
started_at: 2026-01-25T10:00:00Z
ended_at: 2026-01-25T11:00:00Z
agent: coder
---

# Session Summary

## Accomplished
- Implemented feature X
- Fixed bug Y
`
	historyFile := filepath.Join(historyDir, "2026-01-25-test-ses.md")
	os.WriteFile(historyFile, []byte(historyContent), 0644)

	entries, err := LoadRecentHistory(tmpDir, 10)
	if err != nil {
		t.Fatalf("LoadRecentHistory failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if len(entries) > 0 {
		if entries[0].SessionID != "test-session-123" {
			t.Errorf("Expected session ID 'test-session-123', got '%s'", entries[0].SessionID)
		}
	}
}

func TestSaveHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	entry := &HistoryEntry{
		SessionID: "save-test-12345678",
		Date:      time.Now(),
		EndedAt:   time.Now(),
		Agent:     "coder",
		Summary:   "# Session Summary\n\nTest summary.",
	}

	if err := SaveHistory(tmpDir, entry); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	// Verify file was created
	historyDir := filepath.Join(tmpDir, ".edi", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("Failed to read history dir: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 history file, got %d", len(entries))
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}
