//go:build fts5

package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/mcp"
	"github.com/anthropics/aef/codex/internal/storage"
)

// --- Stub implementations of core interfaces ---

type stubEmbedder struct {
	embedFn func(ctx context.Context, text string) ([]float32, error)
}

func (e *stubEmbedder) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return e.embedFn(ctx, text)
}

func (e *stubEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return e.embedFn(ctx, text)
}

func (e *stubEmbedder) Ping(ctx context.Context) error {
	return nil
}

type stubVecStore struct {
	upserted map[string][]float32
	results  []storage.ScoredResult
}

func (v *stubVecStore) Upsert(_ context.Context, id string, vec []float32) error {
	v.upserted[id] = vec
	return nil
}

func (v *stubVecStore) Search(_ context.Context, _ []float32, _ int) ([]storage.ScoredResult, error) {
	return v.results, nil
}

func (v *stubVecStore) Delete(_ context.Context, _ string) error { return nil }

type stubMetadata struct {
	items     map[string]*storage.ItemRecord
	feedback  []*storage.FeedbackRecord
	flightLog []*storage.FlightRecorderRecord
}

func (m *stubMetadata) SaveItem(item *storage.ItemRecord) error {
	m.items[item.ID] = item
	return nil
}

func (m *stubMetadata) GetItem(id string) (*storage.ItemRecord, error) {
	item, ok := m.items[id]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", id)
	}
	return item, nil
}

func (m *stubMetadata) FindByTitle(title string) (*storage.ItemRecord, error) {
	for _, item := range m.items {
		if item.Title == title {
			return item, nil
		}
	}
	return nil, fmt.Errorf("item not found by title: %s", title)
}

func (m *stubMetadata) ListItems(_, _ string, _, _ int) ([]*storage.ItemRecord, error) {
	var items []*storage.ItemRecord
	for _, item := range m.items {
		items = append(items, item)
	}
	return items, nil
}

func (m *stubMetadata) DeleteItem(id string) error {
	delete(m.items, id)
	return nil
}

func (m *stubMetadata) CountItemsByType() (map[string]int, error) {
	counts := make(map[string]int)
	for _, item := range m.items {
		counts[item.Type]++
	}
	return counts, nil
}

func (m *stubMetadata) RecordFeedback(fb *storage.FeedbackRecord) error {
	m.feedback = append(m.feedback, fb)
	return nil
}

func (m *stubMetadata) LogFlightRecorder(entry *storage.FlightRecorderRecord) error {
	m.flightLog = append(m.flightLog, entry)
	return nil
}

func (m *stubMetadata) GetFlightRecorderEntries(sessionID string) ([]*storage.FlightRecorderRecord, error) {
	var entries []*storage.FlightRecorderRecord
	for _, e := range m.flightLog {
		if e.SessionID == sessionID {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (m *stubMetadata) Close() error { return nil }

type stubKeywords struct {
	results []storage.KeywordResult
}

func (k *stubKeywords) KeywordSearch(_ string, _ int) ([]storage.KeywordResult, error) {
	return k.results, nil
}

// --- Test helpers ---

// jsonRPCRequest builds a JSON-RPC request line.
func jsonRPCRequest(id int, method string, params interface{}) []byte {
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

// toolCallRequest builds a tools/call JSON-RPC request.
func toolCallRequest(id int, toolName string, args map[string]interface{}) []byte {
	return jsonRPCRequest(id, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
}

type testServer struct {
	send     func(msg []byte)
	recv     func() (map[string]interface{}, error)
	meta     *stubMetadata
	vecStore *stubVecStore
	cleanup  func()
}

func setupTestServer(t *testing.T, meta *stubMetadata, vecStore *stubVecStore, embedFn func(context.Context, string) ([]float32, error)) *testServer {
	t.Helper()
	if meta == nil {
		meta = &stubMetadata{items: map[string]*storage.ItemRecord{}}
	}
	if vecStore == nil {
		vecStore = &stubVecStore{upserted: map[string][]float32{}}
	}
	if embedFn == nil {
		embedFn = func(_ context.Context, _ string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3}, nil
		}
	}

	engine := core.NewSearchEngineWithDeps(core.SearchEngineDeps{
		Config:   core.Config{},
		VecStore: vecStore,
		Metadata: meta,
		Keywords: &stubKeywords{},
		Embedder: &stubEmbedder{embedFn: embedFn},
		Reranker: nil,
	})

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	server := mcp.NewServer(engine, "test-session")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.RunForIO(ctx, serverR, serverW) }()

	reader := bufio.NewReader(clientR)

	ts := &testServer{
		meta:     meta,
		vecStore: vecStore,
		send: func(msg []byte) {
			_, _ = clientW.Write(msg)
		},
		recv: func() (map[string]interface{}, error) {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}
			var resp map[string]interface{}
			if err := json.Unmarshal(line, &resp); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w (raw: %s)", err, string(line))
			}
			return resp, nil
		},
		cleanup: func() {
			cancel()
			clientW.Close()
			<-done
		},
	}
	return ts
}

// parseToolResult extracts the tool result text from a response and parses it as JSON.
func parseToolResult(resp map[string]interface{}) (map[string]interface{}, bool, error) {
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("no result field")
	}
	isError, _ := result["isError"].(bool)

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return nil, isError, fmt.Errorf("no content")
	}
	first, ok := content[0].(map[string]interface{})
	if !ok {
		return nil, isError, fmt.Errorf("content[0] not an object")
	}
	text, _ := first["text"].(string)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		// Return raw text as a single-key map for error messages
		return map[string]interface{}{"_raw": text}, isError, nil
	}
	return parsed, isError, nil
}

// --- Tests ---

func TestProtocolLifecycle(t *testing.T) {
	ts := setupTestServer(t, nil, nil, nil)
	defer ts.cleanup()

	// 1. Initialize
	ts.send(jsonRPCRequest(1, "initialize", nil))
	resp, err := ts.recv()
	if err != nil {
		t.Fatalf("recv initialize: %v", err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("no result in initialize response: %v", resp)
	}
	if v, _ := result["protocolVersion"].(string); v != "2024-11-05" {
		t.Errorf("protocolVersion = %q, want %q", v, "2024-11-05")
	}
	serverInfo, _ := result["serverInfo"].(map[string]interface{})
	if name, _ := serverInfo["name"].(string); name != "codex" {
		t.Errorf("serverInfo.name = %q, want %q", name, "codex")
	}
	if version, _ := serverInfo["version"].(string); version != "1.0.0" {
		t.Errorf("serverInfo.version = %q, want %q", version, "1.0.0")
	}
	if _, ok := result["capabilities"].(map[string]interface{}); !ok {
		t.Error("missing capabilities in initialize result")
	}

	// 2. notifications/initialized — no response expected
	ts.send(jsonRPCRequest(0, "notifications/initialized", nil))
	// Don't recv — notifications produce no response. Send next request immediately.

	// 3. tools/list
	ts.send(jsonRPCRequest(2, "tools/list", nil))
	resp, err = ts.recv()
	if err != nil {
		t.Fatalf("recv tools/list: %v", err)
	}
	result, ok = resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("no result in tools/list response")
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("tools is not an array: %v", result)
	}
	if len(tools) != 5 {
		t.Errorf("expected 5 tools, got %d", len(tools))
	}

	expectedTools := map[string]bool{
		"recall_search":       false,
		"recall_get":          false,
		"recall_add":          false,
		"recall_feedback":     false,
		"flight_recorder_log": false,
	}
	for _, tool := range tools {
		toolMap, _ := tool.(map[string]interface{})
		name, _ := toolMap["name"].(string)
		desc, _ := toolMap["description"].(string)
		schema := toolMap["inputSchema"]
		if _, ok := expectedTools[name]; ok {
			expectedTools[name] = true
		} else {
			t.Errorf("unexpected tool: %s", name)
		}
		if desc == "" {
			t.Errorf("tool %s has empty description", name)
		}
		if schema == nil {
			t.Errorf("tool %s has nil inputSchema", name)
		}
	}
	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool not found: %s", name)
		}
	}
}

func TestRecallSearchHappyPath(t *testing.T) {
	meta := &stubMetadata{
		items: map[string]*storage.ItemRecord{
			"A": {ID: "A", Type: "pattern", Title: "Retry with jitter", Content: "Add jitter to backoff", Tags: []string{"retry"}, Scope: "project"},
			"B": {ID: "B", Type: "failure", Title: "Thundering herd", Content: "Simultaneous retries", Tags: nil, Scope: "project"},
			"C": {ID: "C", Type: "decision", Title: "Use circuit breaker", Content: "Chose circuit breaker", Tags: nil, Scope: "global"},
		},
	}
	vecStore := &stubVecStore{
		upserted: map[string][]float32{},
		results: []storage.ScoredResult{
			{ID: "A", Score: 0.9},
			{ID: "B", Score: 0.7},
			{ID: "C", Score: 0.5},
		},
	}

	ts := setupTestServer(t, meta, vecStore, nil)
	defer ts.cleanup()

	ts.send(toolCallRequest(1, "recall_search", map[string]interface{}{
		"query": "payment retry",
	}))
	resp, err := ts.recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	parsed, isError, err := parseToolResult(resp)
	if err != nil {
		t.Fatalf("parseToolResult: %v", err)
	}
	if isError {
		t.Fatalf("unexpected isError=true: %v", parsed)
	}

	// Check result structure
	count, _ := parsed["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3", count)
	}

	results, ok := parsed["results"].([]interface{})
	if !ok {
		t.Fatalf("results not an array: %v", parsed)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify first result fields
	first, _ := results[0].(map[string]interface{})
	if rank, _ := first["rank"].(float64); rank != 1 {
		t.Errorf("first result rank = %v, want 1", rank)
	}
	if id, _ := first["id"].(string); id != "A" {
		t.Errorf("first result id = %q, want %q", id, "A")
	}
	if scorePct, _ := first["score_pct"].(string); scorePct != "100%" {
		t.Errorf("first result score_pct = %q, want %q", scorePct, "100%")
	}

	// Verify flight recorder was written (auto-logging)
	if len(meta.flightLog) == 0 {
		t.Error("expected flight recorder entry for search, got none")
	} else {
		entry := meta.flightLog[0]
		if entry.Type != "retrieval_query" {
			t.Errorf("flight log type = %q, want %q", entry.Type, "retrieval_query")
		}
	}
}

func TestRecallSearchValidation(t *testing.T) {
	ts := setupTestServer(t, nil, nil, nil)
	defer ts.cleanup()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "empty args",
			args:    map[string]interface{}{},
			wantErr: "query is required",
		},
		{
			name:    "empty query",
			args:    map[string]interface{}{"query": ""},
			wantErr: "query is required",
		},
		{
			name:    "query exceeds 10KB",
			args:    map[string]interface{}{"query": strings.Repeat("a", 10241)},
			wantErr: "query exceeds maximum size of 10KB",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.send(toolCallRequest(100+i, "recall_search", tt.args))
			resp, err := ts.recv()
			if err != nil {
				t.Fatalf("recv: %v", err)
			}
			parsed, isError, err := parseToolResult(resp)
			if err != nil {
				t.Fatalf("parseToolResult: %v", err)
			}
			if !isError {
				t.Fatalf("expected isError=true, got false")
			}
			raw, _ := parsed["_raw"].(string)
			if !strings.Contains(raw, tt.wantErr) {
				t.Errorf("error text %q does not contain %q", raw, tt.wantErr)
			}
		})
	}
}

func TestRecallAddHappyPath(t *testing.T) {
	t.Setenv("EDI_SESSION_ID", "sess-123")
	t.Setenv("EDI_GIT_BRANCH", "main")
	t.Setenv("EDI_GIT_SHA", "abc123")

	meta := &stubMetadata{items: map[string]*storage.ItemRecord{}}
	vecStore := &stubVecStore{upserted: map[string][]float32{}}
	ts := setupTestServer(t, meta, vecStore, nil)
	defer ts.cleanup()

	ts.send(toolCallRequest(1, "recall_add", map[string]interface{}{
		"type":    "pattern",
		"title":   "retry with jitter",
		"content": "Add jitter to backoff delays",
		"tags":    []interface{}{"retry", "jitter"},
	}))
	resp, err := ts.recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	parsed, isError, err := parseToolResult(resp)
	if err != nil {
		t.Fatalf("parseToolResult: %v", err)
	}
	if isError {
		t.Fatalf("unexpected isError=true: %v", parsed)
	}

	// Verify response has id and message
	id, _ := parsed["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id in response")
	}
	if !strings.HasPrefix(id, "P-") {
		t.Errorf("id %q should start with P- for pattern type", id)
	}
	msg, _ := parsed["message"].(string)
	if !strings.Contains(msg, "pattern") || !strings.Contains(msg, "retry with jitter") {
		t.Errorf("unexpected message: %q", msg)
	}

	// Verify metadata store received the item
	if len(meta.items) != 1 {
		t.Fatalf("expected 1 item in metadata, got %d", len(meta.items))
	}
	var savedItem *storage.ItemRecord
	for _, item := range meta.items {
		savedItem = item
	}
	if savedItem.ID != id {
		t.Errorf("saved item ID = %q, want %q", savedItem.ID, id)
	}
	// Verify env var injection
	if savedItem.Metadata == nil {
		t.Fatal("expected metadata with env vars, got nil")
	}
	if v, _ := savedItem.Metadata["session_id"].(string); v != "sess-123" {
		t.Errorf("metadata.session_id = %q, want %q", v, "sess-123")
	}
	if v, _ := savedItem.Metadata["git_branch"].(string); v != "main" {
		t.Errorf("metadata.git_branch = %q, want %q", v, "main")
	}
	if v, _ := savedItem.Metadata["git_sha"].(string); v != "abc123" {
		t.Errorf("metadata.git_sha = %q, want %q", v, "abc123")
	}

	// Verify vector store received the embedding
	if len(vecStore.upserted) != 1 {
		t.Fatalf("expected 1 vector upserted, got %d", len(vecStore.upserted))
	}
	if _, ok := vecStore.upserted[id]; !ok {
		t.Errorf("vector not upserted for id %q", id)
	}
}

func TestRecallAddValidation(t *testing.T) {
	ts := setupTestServer(t, nil, nil, nil)
	defer ts.cleanup()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "missing type",
			args:    map[string]interface{}{"title": "x", "content": "y"},
			wantErr: "type, title, and content are required",
		},
		{
			name:    "missing title",
			args:    map[string]interface{}{"type": "x", "content": "y"},
			wantErr: "type, title, and content are required",
		},
		{
			name:    "missing content",
			args:    map[string]interface{}{"type": "x", "title": "y"},
			wantErr: "type, title, and content are required",
		},
		{
			name:    "empty content",
			args:    map[string]interface{}{"type": "x", "title": "y", "content": ""},
			wantErr: "type, title, and content are required",
		},
		{
			name:    "content exceeds 1MB",
			args:    map[string]interface{}{"type": "x", "title": "y", "content": strings.Repeat("a", 1<<20+1)},
			wantErr: "content exceeds maximum size of 1MB",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.send(toolCallRequest(200+i, "recall_add", tt.args))
			resp, err := ts.recv()
			if err != nil {
				t.Fatalf("recv: %v", err)
			}
			parsed, isError, err := parseToolResult(resp)
			if err != nil {
				t.Fatalf("parseToolResult: %v", err)
			}
			if !isError {
				t.Fatal("expected isError=true")
			}
			raw, _ := parsed["_raw"].(string)
			if !strings.Contains(raw, tt.wantErr) {
				t.Errorf("error %q does not contain %q", raw, tt.wantErr)
			}
		})
	}
}

func TestRecallAddScopeDefault(t *testing.T) {
	meta := &stubMetadata{items: map[string]*storage.ItemRecord{}}
	ts := setupTestServer(t, meta, nil, nil)
	defer ts.cleanup()

	// No scope provided → should default to "project"
	ts.send(toolCallRequest(1, "recall_add", map[string]interface{}{
		"type": "decision", "title": "test", "content": "test content",
	}))
	resp, err := ts.recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	_, isError, _ := parseToolResult(resp)
	if isError {
		t.Fatal("unexpected error")
	}

	for _, item := range meta.items {
		if item.Scope != "project" {
			t.Errorf("scope = %q, want %q", item.Scope, "project")
		}
	}
}

func TestRecallGetFeedbackFlightRecorder(t *testing.T) {
	meta := &stubMetadata{
		items: map[string]*storage.ItemRecord{
			"P-abc123": {
				ID: "P-abc123", Type: "pattern", Title: "Test Pattern",
				Content: "test content", Scope: "project",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		},
	}
	ts := setupTestServer(t, meta, nil, nil)
	defer ts.cleanup()

	// recall_get happy path
	t.Run("get_happy", func(t *testing.T) {
		ts.send(toolCallRequest(1, "recall_get", map[string]interface{}{"id": "P-abc123"}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, err := parseToolResult(resp)
		if err != nil {
			t.Fatalf("parseToolResult: %v", err)
		}
		if isError {
			t.Fatalf("unexpected error: %v", parsed)
		}
		if id, _ := parsed["id"].(string); id != "P-abc123" {
			t.Errorf("id = %q, want %q", id, "P-abc123")
		}
	})

	// recall_get empty id
	t.Run("get_empty_id", func(t *testing.T) {
		ts.send(toolCallRequest(2, "recall_get", map[string]interface{}{"id": ""}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		_, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true for empty id")
		}
	})

	// recall_feedback happy path
	t.Run("feedback_happy", func(t *testing.T) {
		ts.send(toolCallRequest(3, "recall_feedback", map[string]interface{}{
			"item_id": "P-abc123", "useful": true, "context": "helped avoid bug",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, err := parseToolResult(resp)
		if err != nil {
			t.Fatalf("parseToolResult: %v", err)
		}
		if isError {
			t.Fatalf("unexpected error: %v", parsed)
		}
		if status, _ := parsed["status"].(string); status != "recorded" {
			t.Errorf("status = %q, want %q", status, "recorded")
		}
		if len(meta.feedback) == 0 {
			t.Error("expected feedback to be recorded")
		}
	})

	// recall_feedback missing useful
	t.Run("feedback_missing_useful", func(t *testing.T) {
		ts.send(toolCallRequest(4, "recall_feedback", map[string]interface{}{
			"item_id": "P-abc123",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "useful is required") {
			t.Errorf("error %q does not contain %q", raw, "useful is required")
		}
	})

	// recall_feedback useful not boolean
	t.Run("feedback_useful_not_bool", func(t *testing.T) {
		ts.send(toolCallRequest(5, "recall_feedback", map[string]interface{}{
			"item_id": "P-abc123", "useful": "yes",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "useful must be a boolean") {
			t.Errorf("error %q does not contain %q", raw, "useful must be a boolean")
		}
	})

	// recall_feedback empty item_id
	t.Run("feedback_empty_item_id", func(t *testing.T) {
		ts.send(toolCallRequest(6, "recall_feedback", map[string]interface{}{
			"item_id": "", "useful": true,
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "item_id is required") {
			t.Errorf("error %q does not contain %q", raw, "item_id is required")
		}
	})

	// flight_recorder_log happy path
	t.Run("flight_log_happy", func(t *testing.T) {
		beforeLen := len(meta.flightLog)
		ts.send(toolCallRequest(7, "flight_recorder_log", map[string]interface{}{
			"type": "decision", "content": "chose circuit breaker", "rationale": "lower latency",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, err := parseToolResult(resp)
		if err != nil {
			t.Fatalf("parseToolResult: %v", err)
		}
		if isError {
			t.Fatalf("unexpected error: %v", parsed)
		}
		if status, _ := parsed["status"].(string); status != "logged" {
			t.Errorf("status = %q, want %q", status, "logged")
		}
		if len(meta.flightLog) <= beforeLen {
			t.Error("expected new flight recorder entry")
		}
	})

	// flight_recorder_log validation
	t.Run("flight_log_missing_type", func(t *testing.T) {
		ts.send(toolCallRequest(8, "flight_recorder_log", map[string]interface{}{
			"content": "x",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "type and content are required") {
			t.Errorf("error %q does not contain %q", raw, "type and content are required")
		}
	})

	t.Run("flight_log_missing_content", func(t *testing.T) {
		ts.send(toolCallRequest(9, "flight_recorder_log", map[string]interface{}{
			"type": "decision",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "type and content are required") {
			t.Errorf("error %q does not contain %q", raw, "type and content are required")
		}
	})
}

func TestJSONRPCErrorCodes(t *testing.T) {
	ts := setupTestServer(t, nil, nil, nil)
	defer ts.cleanup()

	t.Run("parse_error", func(t *testing.T) {
		ts.send([]byte("not json\n"))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		errObj, ok := resp["error"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected error object, got: %v", resp)
		}
		code, _ := errObj["code"].(float64)
		if int(code) != -32700 {
			t.Errorf("error code = %v, want -32700", code)
		}
		msg, _ := errObj["message"].(string)
		if msg != "Parse error" {
			t.Errorf("error message = %q, want %q", msg, "Parse error")
		}
	})

	t.Run("method_not_found", func(t *testing.T) {
		ts.send(jsonRPCRequest(1, "foo/bar", nil))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		errObj, ok := resp["error"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected error object, got: %v", resp)
		}
		code, _ := errObj["code"].(float64)
		if int(code) != -32601 {
			t.Errorf("error code = %v, want -32601", code)
		}
	})

	t.Run("invalid_params", func(t *testing.T) {
		// Send tools/call with params as a string instead of object
		raw := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"not-an-object"}` + "\n"
		ts.send([]byte(raw))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		errObj, ok := resp["error"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected error object, got: %v", resp)
		}
		code, _ := errObj["code"].(float64)
		if int(code) != -32602 {
			t.Errorf("error code = %v, want -32602", code)
		}
	})

	t.Run("unknown_tool", func(t *testing.T) {
		ts.send(toolCallRequest(3, "nonexistent_tool", map[string]interface{}{}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		// Unknown tool is a tool-level error (isError), not a protocol error
		_, isError, _ := parseToolResult(resp)
		if !isError {
			t.Error("expected isError=true for unknown tool")
		}
		if resp["error"] != nil {
			t.Error("expected no protocol-level error for unknown tool")
		}
	})
}

func TestEngineErrorPropagation(t *testing.T) {
	embedErr := fmt.Errorf("ollama connection refused")
	failEmbed := func(_ context.Context, _ string) ([]float32, error) {
		return nil, embedErr
	}

	t.Run("search_embed_error", func(t *testing.T) {
		ts := setupTestServer(t, nil, nil, failEmbed)
		defer ts.cleanup()

		ts.send(toolCallRequest(1, "recall_search", map[string]interface{}{"query": "test"}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true for embed error")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "failed to embed query") {
			t.Errorf("error %q does not contain %q", raw, "failed to embed query")
		}
	})

	t.Run("add_embed_error", func(t *testing.T) {
		ts := setupTestServer(t, nil, nil, failEmbed)
		defer ts.cleanup()

		ts.send(toolCallRequest(1, "recall_add", map[string]interface{}{
			"type": "pattern", "title": "test", "content": "test content",
		}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		parsed, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true for embed error")
		}
		raw, _ := parsed["_raw"].(string)
		if !strings.Contains(raw, "embedding failed") {
			t.Errorf("error %q does not contain %q", raw, "embedding failed")
		}
	})

	t.Run("get_not_found", func(t *testing.T) {
		ts := setupTestServer(t, nil, nil, nil)
		defer ts.cleanup()

		ts.send(toolCallRequest(1, "recall_get", map[string]interface{}{"id": "nonexistent"}))
		resp, err := ts.recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		_, isError, _ := parseToolResult(resp)
		if !isError {
			t.Fatal("expected isError=true for not found")
		}
	})
}
