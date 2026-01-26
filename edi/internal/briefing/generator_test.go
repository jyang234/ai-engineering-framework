package briefing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/aef/edi/internal/config"
)

func TestGenerate(t *testing.T) {
	// Note: Cannot use t.Parallel() because Generate() uses os.Getwd()
	tmpDir := t.TempDir()

	// Create .edi directory structure
	ediDir := filepath.Join(tmpDir, ".edi")
	if err := os.MkdirAll(ediDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ediDir, "history"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a profile
	profile := `# Test Project

A test project for unit testing.
`
	if err := os.WriteFile(filepath.Join(ediDir, "profile.md"), []byte(profile), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory - required because Generate() uses os.Getwd()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	cfg := config.DefaultConfig()
	brief, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if brief == nil {
		t.Fatal("Expected non-nil briefing")
	}

	// Render the briefing
	rendered := brief.Render("test-project")
	if rendered == "" {
		t.Error("Expected non-empty briefing")
	}

	// Should include profile content
	if !strings.Contains(rendered, "Project Context") {
		t.Error("Expected briefing to include profile section")
	}
}

func TestLoadRecentHistory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create history directory
	historyDir := filepath.Join(tmpDir, ".edi", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		t.Fatal(err)
	}

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
	if err := os.WriteFile(historyFile, []byte(historyContent), 0644); err != nil {
		t.Fatal(err)
	}

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
	t.Parallel()
	tmpDir := t.TempDir()

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
