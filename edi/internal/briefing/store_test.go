package briefing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveHistoryFull(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	entry := &HistoryEntry{
		SessionID:         "test-session-12345678",
		Date:              time.Now().Add(-time.Hour),
		EndedAt:           time.Now(),
		Agent:             "coder",
		TasksCompleted:    []string{"Task 1", "Task 2"},
		DecisionsCaptured: []string{"Decision A"},
		Summary:           "Implemented feature X with tests.",
	}

	if err := SaveHistory(tmpDir, entry); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	// Verify history directory was created
	historyDir := filepath.Join(tmpDir, ".edi", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Fatal("Expected history directory to be created")
	}

	// Verify file was created with correct naming pattern
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("Failed to read history directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 history file, got %d", len(entries))
	}

	filename := entries[0].Name()
	if !strings.HasSuffix(filename, ".md") {
		t.Errorf("Expected .md extension, got %s", filename)
	}

	// Filename should contain session ID prefix
	if !strings.Contains(filename, "test-ses") {
		t.Errorf("Expected filename to contain session ID prefix, got %s", filename)
	}

	// Read and verify content
	content, err := os.ReadFile(filepath.Join(historyDir, filename))
	if err != nil {
		t.Fatalf("Failed to read history file: %v", err)
	}

	contentStr := string(content)

	// Check frontmatter markers
	if !strings.HasPrefix(contentStr, "---\n") {
		t.Error("Expected file to start with frontmatter")
	}

	if !strings.Contains(contentStr, "session_id: test-session-12345678") {
		t.Error("Expected session_id in frontmatter")
	}

	if !strings.Contains(contentStr, "agent: coder") {
		t.Error("Expected agent in frontmatter")
	}

	// Check body
	if !strings.Contains(contentStr, "Implemented feature X with tests.") {
		t.Error("Expected summary in body")
	}
}

func TestFormatHistoryEntry(t *testing.T) {
	t.Parallel()

	entry := &HistoryEntry{
		SessionID:         "format-test-session",
		Date:              time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		EndedAt:           time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		Agent:             "architect",
		TasksCompleted:    []string{"Design API", "Write spec"},
		DecisionsCaptured: []string{"Use REST over GraphQL"},
		Summary:           "Designed the API structure.",
	}

	result, err := formatHistoryEntry(entry)
	if err != nil {
		t.Fatalf("formatHistoryEntry failed: %v", err)
	}

	// Check structure
	if !strings.HasPrefix(result, "---\n") {
		t.Error("Expected to start with ---")
	}

	if !strings.Contains(result, "\n---\n\n") {
		t.Error("Expected frontmatter to be closed with ---")
	}

	// Check frontmatter content
	if !strings.Contains(result, "session_id: format-test-session") {
		t.Error("Expected session_id in frontmatter")
	}

	if !strings.Contains(result, "agent: architect") {
		t.Error("Expected agent in frontmatter")
	}

	// Check tasks are included
	if !strings.Contains(result, "tasks_completed:") {
		t.Error("Expected tasks_completed in frontmatter")
	}

	// Check decisions are included
	if !strings.Contains(result, "decisions_captured:") {
		t.Error("Expected decisions_captured in frontmatter")
	}

	// Check body
	if !strings.HasSuffix(result, "Designed the API structure.") {
		t.Error("Expected summary to be at the end")
	}
}

func TestFormatHistoryEntryEmptyOptionals(t *testing.T) {
	t.Parallel()

	entry := &HistoryEntry{
		SessionID: "minimal-session",
		Date:      time.Now(),
		EndedAt:   time.Now(),
		Agent:     "coder",
		Summary:   "Minimal entry.",
	}

	result, err := formatHistoryEntry(entry)
	if err != nil {
		t.Fatalf("formatHistoryEntry failed: %v", err)
	}

	// Empty slices should use omitempty
	if strings.Contains(result, "tasks_completed: []") {
		t.Error("Expected empty tasks_completed to be omitted")
	}
}

func TestNewFlightRecorderFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sessionID := "flight-test-12345678"
	fr, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatalf("NewFlightRecorderFile failed: %v", err)
	}
	defer fr.Close()

	// Verify file was created
	if _, err := os.Stat(fr.Path()); os.IsNotExist(err) {
		t.Fatal("Expected flight recorder file to be created")
	}

	// Verify path is in correct location
	expectedDir := filepath.Join(tmpDir, ".edi", "history")
	if !strings.HasPrefix(fr.Path(), expectedDir) {
		t.Errorf("Expected file in %s, got %s", expectedDir, fr.Path())
	}

	// Verify filename contains session prefix (first 8 chars of session ID)
	filename := filepath.Base(fr.Path())
	if !strings.HasPrefix(filename, "flight-t") {
		t.Errorf("Expected filename to start with session prefix, got %s", filename)
	}

	if !strings.HasSuffix(filename, "-flight.jsonl") {
		t.Errorf("Expected filename to end with -flight.jsonl, got %s", filename)
	}
}

func TestFlightRecorderFileWrite(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sessionID := "write-test-12345678"
	fr, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	// Write some entries
	entries := []map[string]interface{}{
		{"type": "decision", "content": "Use Go"},
		{"type": "milestone", "content": "Phase 1 complete"},
		{"type": "error", "content": "Build failed"},
	}

	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		if err := fr.Write(data); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Close to flush
	if err := fr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(fr.Path())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i, err)
		}
	}

	// Verify content
	var entry1 map[string]interface{}
	json.Unmarshal([]byte(lines[0]), &entry1)
	if entry1["type"] != "decision" {
		t.Errorf("Expected first entry type 'decision', got %v", entry1["type"])
	}
}

func TestFlightRecorderFilePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sessionID := "path-test-12345678"
	fr, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	defer fr.Close()

	path := fr.Path()

	// Path should be absolute
	if !filepath.IsAbs(path) {
		t.Error("Expected absolute path")
	}

	// Path should be under project's .edi/history
	if !strings.Contains(path, ".edi") || !strings.Contains(path, "history") {
		t.Errorf("Expected path to contain .edi/history, got %s", path)
	}
}

func TestFlightRecorderFileCreatesDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Remove any .edi directory if exists
	ediDir := filepath.Join(tmpDir, ".edi")
	os.RemoveAll(ediDir)

	sessionID := "createdir-12345678"
	fr, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatalf("NewFlightRecorderFile failed: %v", err)
	}
	defer fr.Close()

	// Verify directory was created
	historyDir := filepath.Join(tmpDir, ".edi", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Error("Expected history directory to be created")
	}
}

func TestFlightRecorderFileAppend(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sessionID := "append-test-12345678"

	// First write
	fr1, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fr1.Write([]byte(`{"entry": 1}`)); err != nil {
		t.Fatal(err)
	}
	fr1.Close()

	// Second write (should append)
	fr2, err := NewFlightRecorderFile(tmpDir, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fr2.Write([]byte(`{"entry": 2}`)); err != nil {
		t.Fatal(err)
	}
	fr2.Close()

	// Verify both entries exist
	content, err := os.ReadFile(fr1.Path())
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (appended), got %d", len(lines))
	}
}
