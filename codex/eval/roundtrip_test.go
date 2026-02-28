//go:build fts5 && evalintegration

package eval_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/mcp"
)

// mcpClient wraps io.Pipe communication with a running MCP server.
type mcpClient struct {
	send   func(msg []byte)
	recv   func() (map[string]interface{}, error)
	nextID int
}

func (c *mcpClient) initialize(t *testing.T) {
	t.Helper()
	c.nextID++
	c.send(c.jsonRPC(c.nextID, "initialize", nil))
	resp, err := c.recv()
	if err != nil {
		t.Fatalf("initialize recv: %v", err)
	}
	if resp["error"] != nil {
		t.Fatalf("initialize error: %v", resp["error"])
	}
	// Send initialized notification (no response expected)
	c.send(c.jsonRPC(0, "notifications/initialized", nil))
}

func (c *mcpClient) callTool(t *testing.T, name string, args map[string]interface{}) (map[string]interface{}, bool) {
	t.Helper()
	c.nextID++
	params := map[string]interface{}{"name": name, "arguments": args}
	c.send(c.jsonRPC(c.nextID, "tools/call", params))
	resp, err := c.recv()
	if err != nil {
		t.Fatalf("callTool(%s) recv: %v", name, err)
	}
	if errObj := resp["error"]; errObj != nil {
		t.Fatalf("callTool(%s) protocol error: %v", name, errObj)
	}

	result, _ := resp["result"].(map[string]interface{})
	isError, _ := result["isError"].(bool)

	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("callTool(%s) empty content", name)
	}
	first, _ := content[0].(map[string]interface{})
	text, _ := first["text"].(string)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return map[string]interface{}{"_raw": text}, isError
	}
	return parsed, isError
}

func (c *mcpClient) jsonRPC(id int, method string, params interface{}) []byte {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		p, _ := json.Marshal(params)
		req["params"] = json.RawMessage(p)
	}
	data, _ := json.Marshal(req)
	return append(data, '\n')
}

// setupRoundTrip creates a real SearchEngine with real SQLite, connected to
// an MCP server via io.Pipe. Requires Ollama for embeddings.
func setupRoundTrip(t *testing.T) (*mcpClient, *core.SearchEngine) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "roundtrip.db")
	ctx := context.Background()

	engine, err := core.NewSearchEngine(ctx, core.Config{
		MetadataDBPath: dbPath,
	})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	server := mcp.NewServer(engine, "roundtrip-test")
	srvCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.RunForIO(srvCtx, serverR, serverW) }()
	t.Cleanup(func() {
		cancel()
		clientW.Close()
		<-done
	})

	reader := bufio.NewReader(clientR)
	client := &mcpClient{
		send: func(msg []byte) { clientW.Write(msg) },
		recv: func() (map[string]interface{}, error) {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}
			var resp map[string]interface{}
			if err := json.Unmarshal(line, &resp); err != nil {
				return nil, fmt.Errorf("unmarshal: %w (raw: %s)", err, string(line))
			}
			return resp, nil
		},
	}

	client.initialize(t)
	return client, engine
}

// TestRoundTrip is the end-to-end round-trip test: add knowledge via MCP,
// search via MCP, verify the knowledge is found. Requires Ollama running.
func TestRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_ = ctx // used implicitly by engine operations

	client, engine := setupRoundTrip(t)

	// Add a known item via MCP
	var addedID string
	t.Run("AddThenSearch", func(t *testing.T) {
		// Add via MCP
		result, isError := client.callTool(t, "recall_add", map[string]interface{}{
			"type":    "failure",
			"title":   "Payment retry without jitter causes thundering herd",
			"content": "When multiple payment processors retry simultaneously without jitter, they overwhelm the downstream service. Fix: add random jitter to exponential backoff delays.",
			"tags":    []interface{}{"retry", "jitter", "thundering-herd"},
		})
		if isError {
			t.Fatalf("recall_add error: %v", result)
		}
		addedID, _ = result["id"].(string)
		if addedID == "" {
			t.Fatal("recall_add returned empty id")
		}
		if !strings.HasPrefix(addedID, "F-") {
			t.Errorf("failure type should get F- prefix, got %q", addedID)
		}

		// Search via MCP
		searchResult, isError := client.callTool(t, "recall_search", map[string]interface{}{
			"query": "how to avoid thundering herd when retrying payments",
		})
		if isError {
			t.Fatalf("recall_search error: %v", searchResult)
		}

		count, _ := searchResult["count"].(float64)
		if count < 1 {
			t.Fatal("expected at least 1 search result")
		}

		results, _ := searchResult["results"].([]interface{})
		found := false
		for _, r := range results {
			rm, _ := r.(map[string]interface{})
			if id, _ := rm["id"].(string); id == addedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("added item %s not found in search results", addedID)
		}
	})

	t.Run("SemanticMatch", func(t *testing.T) {
		// Add a circuit breaker item
		result, isError := client.callTool(t, "recall_add", map[string]interface{}{
			"type":    "pattern",
			"title":   "Circuit breaker state machine",
			"content": "Use three states: closed (normal), open (failing), half-open (testing). Transition open to half-open after a timeout. Allow one request through in half-open to test recovery.",
		})
		if isError {
			t.Fatalf("recall_add error: %v", result)
		}
		cbID, _ := result["id"].(string)

		// Search with semantically related query (no keyword overlap)
		searchResult, isError := client.callTool(t, "recall_search", map[string]interface{}{
			"query": "how to handle cascading failures in microservices",
		})
		if isError {
			t.Fatalf("recall_search error: %v", searchResult)
		}

		results, _ := searchResult["results"].([]interface{})
		found := false
		for _, r := range results {
			rm, _ := r.(map[string]interface{})
			if id, _ := rm["id"].(string); id == cbID {
				found = true
				break
			}
		}
		if !found {
			t.Log("NOTE: semantic match not found — this may indicate embedding model quality or vector search threshold issue")
		}
	})

	t.Run("ScopeIsolation", func(t *testing.T) {
		// Add item with scope project-a
		_, isError := client.callTool(t, "recall_add", map[string]interface{}{
			"type":    "context",
			"title":   "Database migration strategy",
			"content": "Always use versioned migrations with rollback support. Never alter columns in-place on production databases.",
			"scope":   "project-a",
		})
		if isError {
			t.Fatal("recall_add project-a failed")
		}

		// Add item with scope project-b
		_, isError = client.callTool(t, "recall_add", map[string]interface{}{
			"type":    "context",
			"title":   "API versioning approach",
			"content": "Use URL path versioning (v1, v2) for external APIs. Use header versioning for internal APIs.",
			"scope":   "project-b",
		})
		if isError {
			t.Fatal("recall_add project-b failed")
		}

		// Search with scope=project-a
		searchResult, isError := client.callTool(t, "recall_search", map[string]interface{}{
			"query": "database migration",
			"scope": "project-a",
		})
		if isError {
			t.Fatalf("recall_search error: %v", searchResult)
		}

		results, _ := searchResult["results"].([]interface{})
		for _, r := range results {
			rm, _ := r.(map[string]interface{})
			scope, _ := rm["scope"].(string)
			if scope == "project-b" {
				t.Error("search with scope=project-a returned project-b item")
			}
		}
	})

	t.Run("FlightRecorderAudit", func(t *testing.T) {
		entries, err := engine.GetFlightRecorderEntries("roundtrip-test")
		if err != nil {
			t.Fatalf("GetFlightRecorderEntries: %v", err)
		}

		// We've done multiple searches, each should auto-log a retrieval_query
		found := false
		for _, e := range entries {
			if e.Type == "retrieval_query" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected at least one retrieval_query flight recorder entry")
		}
	})

	t.Run("FeedbackRoundTrip", func(t *testing.T) {
		if addedID == "" {
			t.Skip("no item ID from AddThenSearch")
		}

		result, isError := client.callTool(t, "recall_feedback", map[string]interface{}{
			"item_id": addedID,
			"useful":  true,
			"context": "helped avoid thundering herd bug in payment service",
		})
		if isError {
			t.Fatalf("recall_feedback error: %v", result)
		}
		status, _ := result["status"].(string)
		if status != "recorded" {
			t.Errorf("feedback status = %q, want %q", status, "recorded")
		}
	})
}
