//go:build integration

package recall

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/aef/edi/internal/testutil"
)

// createTestDatabase creates a RECALL database with the given items.
func createTestDatabase(t *testing.T, dir string, items []testutil.RecallItem) string {
	t.Helper()

	dbPath := filepath.Join(dir, "recall.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer storage.Close()

	now := time.Now()
	for _, item := range items {
		recallItem := &Item{
			ID:        item.ID,
			Type:      item.Type,
			Title:     item.Title,
			Content:   item.Content,
			Tags:      item.Tags,
			Scope:     item.Scope,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := storage.Add(recallItem); err != nil {
			t.Fatalf("Failed to add test item %s: %v", item.ID, err)
		}
	}

	return dbPath
}

// getEdiBinary returns the path to the edi binary.
// It looks for the binary in common locations. The binary should be built
// before running integration tests (make test-integration handles this).
func getEdiBinary(t *testing.T) string {
	t.Helper()

	// Get the current working directory to construct absolute paths
	cwd, _ := os.Getwd()

	// Look for binary relative to test file location
	// Tests are in internal/recall/, binary is in bin/
	binPaths := []string{
		filepath.Join(cwd, "..", "..", "bin", "edi"),
		filepath.Join(cwd, "bin", "edi"),
	}

	for _, binPath := range binPaths {
		absPath, _ := filepath.Abs(binPath)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	// Try to find via PATH
	if path, err := exec.LookPath("edi"); err == nil {
		return path
	}

	t.Fatal("edi binary not found. Run 'make build' first or ensure edi is in PATH")
	return ""
}

func TestMCPProtocolLifecycle(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Create a database for testing
	dbPath := filepath.Join(env.ProjectEDI, "recall", "test.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Get edi binary
	ediBinary := getEdiBinary(t)

	// Start the RECALL server
	client, err := testutil.NewMCPTestClient(
		ediBinary,
		"recall-server",
		"--project-db", dbPath,
		"--session-id", "test-session-001",
	)
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer client.Close()

	// Test 1: Initialize handshake
	t.Run("Initialize", func(t *testing.T) {
		result, err := client.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if result.ServerInfo.Name != "recall" {
			t.Errorf("Expected server name 'recall', got '%s'", result.ServerInfo.Name)
		}
		if result.ProtocolVersion != "2024-11-05" {
			t.Errorf("Expected protocol version '2024-11-05', got '%s'", result.ProtocolVersion)
		}
	})

	// Test 2: List tools
	t.Run("ListTools", func(t *testing.T) {
		tools, err := client.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 5 {
			t.Errorf("Expected 5 tools, got %d", len(tools))
		}

		expectedTools := map[string]bool{
			"recall_search":       false,
			"recall_get":          false,
			"recall_add":          false,
			"recall_feedback":     false,
			"flight_recorder_log": false,
		}

		for _, tool := range tools {
			if _, exists := expectedTools[tool.Name]; exists {
				expectedTools[tool.Name] = true
			}
		}

		for name, found := range expectedTools {
			if !found {
				t.Errorf("Expected tool '%s' not found", name)
			}
		}
	})

	// Test 3: Search empty database
	t.Run("SearchEmptyDB", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "test query",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		count := result["count"].(float64)
		if count != 0 {
			t.Errorf("Expected 0 results in empty DB, got %v", count)
		}
	})

	// Test 4: Add an item
	var addedID string
	t.Run("AddItem", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_add", map[string]interface{}{
			"type":    "pattern",
			"title":   "Test Pattern",
			"content": "This is a test pattern for integration testing.",
			"tags":    []string{"test", "integration"},
			"scope":   "project",
		})
		if err != nil {
			t.Fatalf("recall_add failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		addedID = result["id"].(string)
		if addedID == "" {
			t.Error("Expected non-empty ID from recall_add")
		}
		if !hasPrefix(addedID, "P-") {
			t.Errorf("Expected ID to start with 'P-' for pattern, got '%s'", addedID)
		}
	})

	// Test 5: Search and find the item
	t.Run("SearchFindsItem", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "test pattern integration",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		count := result["count"].(float64)
		if count != 1 {
			t.Errorf("Expected 1 result, got %v", count)
		}

		results := result["results"].([]interface{})
		if len(results) != 1 {
			t.Fatalf("Expected 1 result in array, got %d", len(results))
		}

		item := results[0].(map[string]interface{})
		if item["id"] != addedID {
			t.Errorf("Expected ID '%s', got '%v'", addedID, item["id"])
		}
	})

	// Test 6: Get item by ID
	t.Run("GetItem", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_get", map[string]interface{}{
			"id": addedID,
		})
		if err != nil {
			t.Fatalf("recall_get failed: %v", err)
		}

		var item map[string]interface{}
		if err := json.Unmarshal([]byte(text), &item); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if item["title"] != "Test Pattern" {
			t.Errorf("Expected title 'Test Pattern', got '%v'", item["title"])
		}
		if item["type"] != "pattern" {
			t.Errorf("Expected type 'pattern', got '%v'", item["type"])
		}
	})

	// Test 7: Record feedback
	t.Run("RecordFeedback", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_feedback", map[string]interface{}{
			"item_id": addedID,
			"useful":  true,
			"context": "Found this helpful during integration testing",
		})
		if err != nil {
			t.Fatalf("recall_feedback failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if result["status"] != "recorded" {
			t.Errorf("Expected status 'recorded', got '%v'", result["status"])
		}
	})

	// Test 8: Flight recorder log
	t.Run("FlightRecorderLog", func(t *testing.T) {
		text, err := client.CallToolRaw("flight_recorder_log", map[string]interface{}{
			"type":      "decision",
			"content":   "Decided to use FTS5 for search",
			"rationale": "Good performance and built into SQLite",
			"metadata": map[string]interface{}{
				"files_affected": []string{"storage.go", "server.go"},
			},
		})
		if err != nil {
			t.Fatalf("flight_recorder_log failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if result["status"] != "logged" {
			t.Errorf("Expected status 'logged', got '%v'", result["status"])
		}
	})
}

func TestMCPErrorHandling(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	dbPath := filepath.Join(env.ProjectEDI, "recall", "test.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	ediBinary := getEdiBinary(t)

	client, err := testutil.NewMCPTestClient(
		ediBinary,
		"recall-server",
		"--project-db", dbPath,
		"--session-id", "test-session-002",
	)
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer client.Close()

	// Initialize first
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test: Invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		resp, err := client.SendRaw([]byte(`{invalid json`))
		if err != nil {
			t.Fatalf("SendRaw failed: %v", err)
		}

		if resp.Error == nil {
			t.Error("Expected error for invalid JSON")
		} else if resp.Error.Code != -32700 {
			t.Errorf("Expected error code -32700 (Parse error), got %d", resp.Error.Code)
		}
	})

	// Test: Unknown method
	t.Run("UnknownMethod", func(t *testing.T) {
		msg := `{"jsonrpc":"2.0","id":100,"method":"unknown/method"}`
		resp, err := client.SendRaw([]byte(msg))
		if err != nil {
			t.Fatalf("SendRaw failed: %v", err)
		}

		if resp.Error == nil {
			t.Error("Expected error for unknown method")
		} else if resp.Error.Code != -32601 {
			t.Errorf("Expected error code -32601 (Method not found), got %d", resp.Error.Code)
		}
	})

	// Test: Unknown tool
	t.Run("UnknownTool", func(t *testing.T) {
		result, err := client.CallTool("unknown_tool", map[string]interface{}{})
		if err == nil {
			// The error might be returned in the result
			if result != nil && !result.IsError {
				t.Error("Expected error for unknown tool")
			}
		}
		// If err is returned, that's also valid
	})

	// Test: Missing required params
	t.Run("MissingRequiredParams", func(t *testing.T) {
		result, err := client.CallTool("recall_search", map[string]interface{}{
			// missing "query" parameter
		})
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		if !result.IsError {
			t.Error("Expected IsError=true for missing required params")
		}
	})

	// Test: Invalid param types
	t.Run("InvalidParamTypes", func(t *testing.T) {
		result, err := client.CallTool("recall_search", map[string]interface{}{
			"query": 12345, // should be string
		})
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		// Empty query should result in error
		if !result.IsError {
			t.Log("Note: Server accepted numeric query - may need stricter validation")
		}
	})
}

func TestFTS5SearchIntegration(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Create database with sample data
	items := testutil.AllSampleItems()
	dbPath := createTestDatabase(t, filepath.Join(env.ProjectEDI, "recall"), items)

	ediBinary := getEdiBinary(t)

	client, err := testutil.NewMCPTestClient(
		ediBinary,
		"recall-server",
		"--project-db", dbPath,
		"--session-id", "test-session-003",
	)
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer client.Close()

	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test: Exact match query
	t.Run("ExactMatch", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "Error Handling Pattern",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		count := int(result["count"].(float64))
		if count < 1 {
			t.Errorf("Expected at least 1 result for exact match, got %d", count)
		}
	})

	// Test: Partial match query
	t.Run("PartialMatch", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "error",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		count := int(result["count"].(float64))
		if count < 1 {
			t.Errorf("Expected at least 1 result for partial match, got %d", count)
		}
	})

	// Test: Multi-word query
	t.Run("MultiWordQuery", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "database connection pooling",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		count := int(result["count"].(float64))
		if count < 1 {
			t.Errorf("Expected at least 1 result for multi-word query, got %d", count)
		}
	})

	// Test: Type filtering
	t.Run("TypeFiltering", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "SQLite",
			"types": []string{"decision"},
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		results := result["results"].([]interface{})
		for _, r := range results {
			item := r.(map[string]interface{})
			if item["type"] != "decision" {
				t.Errorf("Expected only 'decision' type, got '%v'", item["type"])
			}
		}
	})

	// Test: Scope filtering
	t.Run("ScopeFiltering", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "pattern",
			"scope": "global",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		results := result["results"].([]interface{})
		for _, r := range results {
			item := r.(map[string]interface{})
			if item["scope"] != "global" {
				t.Errorf("Expected only 'global' scope, got '%v'", item["scope"])
			}
		}
	})

	// Test: Limit results
	t.Run("LimitResults", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "pattern OR decision OR error",
			"limit": 2,
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		results := result["results"].([]interface{})
		if len(results) > 2 {
			t.Errorf("Expected at most 2 results with limit, got %d", len(results))
		}
	})
}

func TestMultiScopeSearch(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Create items with different scopes
	items := []testutil.RecallItem{
		{
			ID:      "P-global01",
			Type:    "pattern",
			Title:   "Global Auth Pattern",
			Content: "Global authentication pattern for all projects.",
			Tags:    []string{"auth", "global"},
			Scope:   "global",
		},
		{
			ID:      "P-project01",
			Type:    "pattern",
			Title:   "Project Auth Pattern",
			Content: "Project-specific authentication pattern.",
			Tags:    []string{"auth", "project"},
			Scope:   "project",
		},
		{
			ID:      "P-global02",
			Type:    "pattern",
			Title:   "Global Logging Pattern",
			Content: "Global logging pattern for all projects.",
			Tags:    []string{"logging", "global"},
			Scope:   "global",
		},
	}

	dbPath := createTestDatabase(t, filepath.Join(env.ProjectEDI, "recall"), items)
	ediBinary := getEdiBinary(t)

	client, err := testutil.NewMCPTestClient(
		ediBinary,
		"recall-server",
		"--project-db", dbPath,
		"--session-id", "test-session-004",
	)
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer client.Close()

	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test: Search with scope=all returns both
	t.Run("ScopeAll", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "authentication pattern",
			"scope": "all",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		count := int(result["count"].(float64))
		if count < 2 {
			t.Errorf("Expected at least 2 results with scope=all, got %d", count)
		}

		// Verify both scopes are present
		results := result["results"].([]interface{})
		hasGlobal := false
		hasProject := false
		for _, r := range results {
			item := r.(map[string]interface{})
			if item["scope"] == "global" {
				hasGlobal = true
			}
			if item["scope"] == "project" {
				hasProject = true
			}
		}

		if !hasGlobal {
			t.Error("Expected global scope item in results")
		}
		if !hasProject {
			t.Error("Expected project scope item in results")
		}
	})

	// Test: Search with scope=global returns only global
	t.Run("ScopeGlobalOnly", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "pattern",
			"scope": "global",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		results := result["results"].([]interface{})
		for _, r := range results {
			item := r.(map[string]interface{})
			if item["scope"] != "global" {
				t.Errorf("Expected only global scope, got '%v'", item["scope"])
			}
		}
	})

	// Test: Search with scope=project returns only project
	t.Run("ScopeProjectOnly", func(t *testing.T) {
		text, err := client.CallToolRaw("recall_search", map[string]interface{}{
			"query": "pattern",
			"scope": "project",
		})
		if err != nil {
			t.Fatalf("recall_search failed: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal([]byte(text), &result)

		results := result["results"].([]interface{})
		for _, r := range results {
			item := r.(map[string]interface{})
			if item["scope"] != "project" {
				t.Errorf("Expected only project scope, got '%v'", item["scope"])
			}
		}
	})
}

// hasPrefix checks if string has the given prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
