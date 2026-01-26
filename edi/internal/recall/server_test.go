package recall

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test Add
	item := &Item{
		ID:        "P-test001",
		Type:      "pattern",
		Title:     "Test Pattern",
		Content:   "This is a test pattern for retry logic",
		Tags:      []string{"retry", "testing"},
		Scope:     "project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := storage.Add(item); err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}

	// Test Get
	retrieved, err := storage.Get("P-test001")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if retrieved.Title != "Test Pattern" {
		t.Errorf("Expected title 'Test Pattern', got '%s'", retrieved.Title)
	}

	// Test Search
	results, err := storage.Search("retry", nil, "", 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Test Flight Recorder
	entry := &FlightRecorderEntry{
		SessionID: "test-session",
		Timestamp: time.Now(),
		Type:      "decision",
		Content:   "Test decision",
		Rationale: "Test rationale",
		Metadata:  map[string]interface{}{"key": "value"},
	}

	if err := storage.LogFlightRecorder(entry); err != nil {
		t.Fatalf("Failed to log flight recorder: %v", err)
	}

	entries, err := storage.GetFlightRecorderEntries("test-session")
	if err != nil {
		t.Fatalf("Failed to get flight recorder entries: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestMCPProtocol(t *testing.T) {
	t.Parallel()

	// Test MCP message parsing
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`

	var req MCPRequest
	if err := json.Unmarshal([]byte(initMsg), &req); err != nil {
		t.Fatalf("Failed to parse init message: %v", err)
	}

	if req.Method != "initialize" {
		t.Errorf("Expected method 'initialize', got '%s'", req.Method)
	}

	// Test tools/list message
	listMsg := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`

	if err := json.Unmarshal([]byte(listMsg), &req); err != nil {
		t.Fatalf("Failed to parse list message: %v", err)
	}

	if req.Method != "tools/list" {
		t.Errorf("Expected method 'tools/list', got '%s'", req.Method)
	}
}
