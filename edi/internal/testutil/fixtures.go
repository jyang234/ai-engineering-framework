package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// RecallItem is a test data structure for RECALL items.
// This is separate from recall.Item to avoid import cycles.
type RecallItem struct {
	ID      string
	Type    string
	Title   string
	Content string
	Tags    []string
	Scope   string
}

// SamplePatternItems returns sample pattern items for testing.
func SamplePatternItems() []RecallItem {
	return []RecallItem{
		{
			ID:      "P-test0001",
			Type:    "pattern",
			Title:   "Error Handling Pattern",
			Content: "Always wrap errors with context using fmt.Errorf with %w verb for proper error chaining.",
			Tags:    []string{"go", "errors", "best-practice"},
			Scope:   "global",
		},
		{
			ID:      "P-test0002",
			Type:    "pattern",
			Title:   "Database Connection Pattern",
			Content: "Use connection pooling and always defer Close() immediately after opening a connection.",
			Tags:    []string{"database", "go", "sqlite"},
			Scope:   "project",
		},
		{
			ID:      "P-test0003",
			Type:    "pattern",
			Title:   "Test Setup Pattern",
			Content: "Use t.Helper() in test helpers. Create temp directories with t.TempDir() for automatic cleanup.",
			Tags:    []string{"testing", "go"},
			Scope:   "global",
		},
	}
}

// SampleDecisionItems returns sample decision items for testing.
func SampleDecisionItems() []RecallItem {
	return []RecallItem{
		{
			ID:      "D-test0001",
			Type:    "decision",
			Title:   "Use SQLite for v0",
			Content: "Decision: Use SQLite with FTS5 for knowledge storage in v0. Rationale: Simple, no external dependencies, good enough for initial use case.",
			Tags:    []string{"architecture", "storage"},
			Scope:   "project",
		},
		{
			ID:      "D-test0002",
			Type:    "decision",
			Title:   "Go over Python",
			Content: "Decision: Use Go for EDI CLI instead of Python. Rationale: Single binary distribution, official MCP SDK support, better performance.",
			Tags:    []string{"architecture", "language"},
			Scope:   "global",
		},
	}
}

// SampleFailureItems returns sample failure items for testing.
func SampleFailureItems() []RecallItem {
	return []RecallItem{
		{
			ID:      "F-test0001",
			Type:    "failure",
			Title:   "FTS5 build tag missing",
			Content: "Build failed with 'no such module: fts5'. Solution: Add -tags fts5 to go build command.",
			Tags:    []string{"sqlite", "fts5", "build"},
			Scope:   "project",
		},
	}
}

// AllSampleItems returns all sample items combined.
func AllSampleItems() []RecallItem {
	var all []RecallItem
	all = append(all, SamplePatternItems()...)
	all = append(all, SampleDecisionItems()...)
	all = append(all, SampleFailureItems()...)
	return all
}

// ClaudeTask represents a Claude Code task for testing.
type ClaudeTask struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Blocks      []string `json:"blocks,omitempty"`
	BlockedBy   []string `json:"blockedBy,omitempty"`
}

// CreateClaudeTaskFile creates a Claude task JSON file in the given directory.
func CreateClaudeTaskFile(t *testing.T, dir string, task ClaudeTask) {
	t.Helper()

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.json", task.ID))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}
}

// SampleClaudeTasks returns sample Claude tasks for testing.
func SampleClaudeTasks() []ClaudeTask {
	return []ClaudeTask{
		{
			ID:          "1",
			Subject:     "Implement MCP test client",
			Description: "Create a test client for MCP protocol testing",
			Status:      "pending",
		},
		{
			ID:          "2",
			Subject:     "Write integration tests",
			Description: "Write integration tests for RECALL server",
			Status:      "in_progress",
			BlockedBy:   []string{"1"},
		},
		{
			ID:          "3",
			Subject:     "Update documentation",
			Description: "Update documentation with test instructions",
			Status:      "pending",
			BlockedBy:   []string{"2"},
		},
	}
}

// SampleManifestYAML returns sample manifest YAML for testing.
func SampleManifestYAML() string {
	return `version: 1
last_session_id: test-session-123
tasks:
  - id: "1"
    subject: Implement MCP test client
    description: Create a test client for MCP protocol testing
    status: pending
  - id: "2"
    subject: Write integration tests
    description: Write integration tests for RECALL server
    status: in_progress
    blockedBy:
      - "1"
`
}
