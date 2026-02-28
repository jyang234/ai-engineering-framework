# Test Spec: Filling the Gaps

> Date: 2026-02-27
> Question: Are the tools built well and built in a way that allows Claude to use them effectively?
> Scope: 5 untested areas identified in the audit

---

## Design Principles (Learned from What Works)

The best tests in this codebase share these properties:

1. **Test real behavior, not mocks.** The PayFlow harness indexes 30 real documents, searches with real embeddings, and asserts real IR metrics. The vecstore tests use real SQLite. The RECALL integration tests spawn a real subprocess. Copy this.

2. **Assert outcomes, not structure.** "Recall@10 >= 0.3" proves the search works. "cond.Name == baseline" proves you typed a string correctly. Test what the tool *does*, not what its fields contain.

3. **One expensive setup, many cheap assertions.** `harness_test.go` indexes once, then runs 6 subtests against the indexed state. Don't rebuild the world for each assertion.

4. **Proportional complexity.** `fusion_test.go` proves RRF correct in 171 lines. Don't write 600 lines to test a 200-line component.

5. **Self-contained when possible.** Use `io.Pipe` over subprocesses when you don't need to test binary startup. Use `t.TempDir()` for isolation. Only require Ollama for embedding tests.

---

## Gap 1: MCP Server (`codex/internal/mcp/`)

### What it does

The MCP server is the JSON-RPC interface between Claude Code and RECALL. It handles protocol negotiation, routes tool calls to the search engine, validates parameters, and formats responses. It's the only way Claude Code talks to RECALL.

### Mock strategy

The `ToolHandler` takes a concrete `*core.SearchEngine`, not an interface. But `SearchEngine` is constructed via `core.NewSearchEngineWithDeps(core.SearchEngineDeps{...})` which accepts interfaces. The mocks in `core/mocks_test.go` are in `_test.go` files and **cannot be imported from the `mcp` package**.

**Solution:** Create minimal mock implementations of these interfaces directly in `server_test.go`:

```go
// In server_test.go (package mcp or mcp_test)

type stubEmbedder struct {
    embedFn func(ctx context.Context, text string) ([]float32, error)
}
func (e *stubEmbedder) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
    return e.embedFn(ctx, text)
}
func (e *stubEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
    return e.embedFn(ctx, text)
}

type stubVecStore struct {
    upserted map[string][]float32
    results  []storage.ScoredResult
}
func (v *stubVecStore) Upsert(ctx context.Context, id string, vec []float32) error {
    v.upserted[id] = vec; return nil
}
func (v *stubVecStore) Search(ctx context.Context, q []float32, limit int) ([]storage.ScoredResult, error) {
    return v.results, nil
}
func (v *stubVecStore) Delete(ctx context.Context, id string) error { return nil }

type stubMetadata struct {
    items      map[string]*storage.ItemRecord
    feedback   []*storage.FeedbackRecord
    flightLog  []*storage.FlightRecorderRecord
}
// Implement all MetadataStorage methods: SaveItem, GetItem, FindByTitle,
// ListItems, DeleteItem, CountItemsByType, RecordFeedback,
// LogFlightRecorder, GetFlightRecorderEntries, Close

type stubKeywords struct {
    results []storage.KeywordResult
}
func (k *stubKeywords) KeywordSearch(query string, limit int) ([]storage.KeywordResult, error) {
    return k.results, nil
}
```

Then construct the engine:

```go
engine := core.NewSearchEngineWithDeps(core.SearchEngineDeps{
    Config:   core.Config{},
    VecStore: &stubVecStore{upserted: map[string][]float32{}, results: vecResults},
    Metadata: &stubMetadata{items: map[string]*storage.ItemRecord{}},
    Keywords: &stubKeywords{},
    Embedder: &stubEmbedder{embedFn: func(_ context.Context, _ string) ([]float32, error) {
        return []float32{0.1, 0.2, 0.3}, nil
    }},
    Reranker: nil,
})
server := mcp.NewServer(engine, "test-session")
```

### Test harness pattern

Use `io.Pipe` + goroutine for the server, write JSON-RPC messages from the test goroutine:

```go
func setupTestServer(t *testing.T, engine *core.SearchEngine) (send func(msg []byte), recv func() []byte, cleanup func()) {
    t.Helper()
    clientR, serverW := io.Pipe()
    serverR, clientW := io.Pipe()

    server := NewServer(engine, "test-session-id")
    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan error, 1)
    go func() { done <- server.RunForIO(ctx, serverR, serverW) }()

    reader := bufio.NewReader(clientR)

    send = func(msg []byte) {
        clientW.Write(append(msg, '\n'))
    }
    recv = func() []byte {
        line, _ := reader.ReadBytes('\n')
        return line
    }
    cleanup = func() {
        cancel()
        clientW.Close()
        <-done
    }
    return
}
```

### What to test

**File: `codex/internal/mcp/server_test.go`**

**Build tag: `//go:build fts5`** (required because `core` imports `storage` which uses FTS5)

#### Test 1: Protocol lifecycle

```
Setup: Create Server with mock engine, connect via io.Pipe
Test:
  1. Send initialize → expect:
     - result.protocolVersion == "2024-11-05"
     - result.serverInfo.name == "codex"
     - result.serverInfo.version == "1.0.0"
     - result.capabilities.tools exists
  2. Send notifications/initialized → no response (notification returns nil)
  3. Send tools/list → expect 5 tools:
     recall_search, recall_get, recall_add, recall_feedback, flight_recorder_log
  4. Each tool has name, description (non-empty string), inputSchema (non-nil)
Assert: Protocol completes without error, tool count and names correct
```

#### Test 2: recall_search happy path

```
Setup:
  - stubVecStore returns 3 scored results: IDs ["A","B","C"] scores [0.9, 0.7, 0.5]
  - stubMetadata.GetItem returns ItemRecord for each ID with type, title, content
  - stubEmbedder returns []float32{0.1, 0.2, 0.3} for any input
Test: Send tools/call recall_search with arguments {"query": "payment retry"}
Assert:
  - Response is not an error (no Error field)
  - Parse result.content[0].text as JSON
  - JSON has "results" array with 3 items, "count": 3
  - Each result has: rank, id, type, title, content, tags, score, score_pct
  - score_pct for top = 100%, computed as (score / topScore) * 100
  - stubMetadata.flightLog has 1 entry with type "retrieval_query"
```

#### Test 3: recall_search parameter validation (table-driven)

```
Tests:
  | Arguments                       | Expected error text                |
  |---------------------------------|------------------------------------|
  | {}                              | "query is required"                |
  | {"query": ""}                   | "query is required"                |
  | {"query": <10241-byte string>}  | "query exceeds maximum size of 10KB" |
  | {"query": "ok", "limit": 5}     | no error; engine called with limit 5 |
  | {"query": "ok"}                 | no error; engine called with limit 10 (default) |
  | {"query": "ok", "types": ["pattern"]} | no error; engine called with types |
  | {"query": "ok", "scope": "proj"} | no error; engine called with scope |

For error cases: result.isError == true, content[0].text contains error string
For success cases: result.isError is false or absent
```

#### Test 4: recall_add happy path + env var injection

```
Setup:
  - t.Setenv("EDI_SESSION_ID", "sess-123")
  - t.Setenv("EDI_GIT_BRANCH", "main")
  - t.Setenv("EDI_GIT_SHA", "abc123")
  - stubEmbedder returns []float32{0.1, 0.2, 0.3}
Test: Send recall_add with {
  "type": "pattern", "title": "retry with jitter",
  "content": "Add jitter to backoff", "tags": ["retry"]
}
Assert:
  - No error
  - stubMetadata.items has 1 entry
  - That entry's ID starts with "P-" (pattern prefix)
  - That entry's Metadata contains: session_id="sess-123", git_branch="main", git_sha="abc123"
  - stubVecStore.upserted has 1 entry with matching ID
  - Response JSON has "id" and "message" fields
Cleanup: env vars cleaned by t.Setenv automatically
```

#### Test 5: recall_add parameter validation (table-driven)

```
Tests:
  | Arguments                                  | Expected error text                  |
  |---------------------------------------------|--------------------------------------|
  | {"title": "x", "content": "y"}             | "type, title, and content are required" |
  | {"type": "x", "content": "y"}              | "type, title, and content are required" |
  | {"type": "x", "title": "y"}                | "type, title, and content are required" |
  | {"type": "x", "title": "y", "content": ""} | "type, title, and content are required" |
  | {"type": "x", "title": "y", "content": <1MB+1 bytes>} | "content exceeds maximum size of 1MB" |
  | {"type": "x", "title": "y", "content": "z"} | no error; scope defaults to "project" |
  | {"type": "x", "title": "y", "content": "z", "scope": "team"} | no error; scope is "team" |
```

#### Test 6: recall_get, recall_feedback, flight_recorder_log (table-driven)

```
recall_get:
  - {"id": "P-abc123"} with item in stubMetadata → returns item data
  - {"id": ""} → isError: true, "id is required"
  - {} → isError: true (id not present, empty string extraction)

recall_feedback:
  - {"item_id": "P-abc", "useful": true} → stubMetadata.feedback has 1 entry
  - {"item_id": "P-abc", "useful": true, "context": "helped"} → feedback.Context = "helped"
  - {} → isError: true, "useful is required"
  - {"useful": true} → isError: true, "item_id is required"
  - {"item_id": "P-abc", "useful": "yes"} → isError: true, "useful must be a boolean"

flight_recorder_log:
  - {"type": "decision", "content": "chose X"} → stubMetadata.flightLog has entry
  - {"type": "decision", "content": "chose X", "rationale": "because Y"} → entry.Rationale = "because Y"
  - {"content": "x"} → isError: true, "type and content are required"
  - {"type": "x"} → isError: true, "type and content are required"
```

#### Test 7: JSON-RPC error codes

```
Tests (send raw bytes, parse response):
  - Send "not json\n" → error code -32700, message "Parse error"
  - Send {"jsonrpc":"2.0","id":1,"method":"foo/bar"} → error code -32601, "Method not found"
  - Send {"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"} → error code -32602, "Invalid params"
  - Send valid tools/call with unknown tool name → isError: true in result (NOT a protocol error)
```

#### Test 8: Engine error propagation

```
Setup: stubEmbedder returns error; stubMetadata returns error; stubVecStore returns error
Test:
  - recall_search with embedder error → isError: true, text contains "failed to embed query"
  - recall_get with metadata error → isError: true, text contains engine error
  - recall_add with embedder error → isError: true, text contains "embedding failed"
Assert: Tool-level errors become isError: true with "Error: ..." text, not protocol errors
```

### Estimated size

~450 lines. 8 test functions. No external dependencies beyond `io.Pipe` + stubs.

---

## Gap 2: Embedding Client (`codex/internal/embedding/`)

### What it does

`LocalClient` calls an Ollama-compatible HTTP endpoint to generate vector embeddings. It prefixes queries with "search_query: " and documents with "search_document: " for asymmetric retrieval. It retries on 5xx errors with exponential backoff. Constants: `localMaxRetries = 5`, `localInitialDelay = 1 * time.Second`.

### What to test

**File: `codex/internal/embedding/local_test.go`**

**No build tags needed** — this package has no SQLite/FTS5 dependency.

Use `httptest.NewServer` to mock Ollama — no real Ollama needed.

#### Helper: mock Ollama server

```go
// Returns a test server and a pointer to an atomic request counter.
// responseFunc controls what the server returns.
func mockOllama(t *testing.T, responseFunc func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(responseFunc))
}

// Standard successful response body:
// {"embeddings": [[0.1, 0.2, 0.3]]}
//
// Ollama request body shape:
// {"model": "nomic-embed-text", "input": "search_query: hello world"}
```

#### Test 1: EmbedDocument and EmbedQuery prefixes

```
Setup: httptest server that records request body JSON, returns {"embeddings": [[0.1, 0.2, 0.3]]}
Test:
  - Call EmbedDocument(ctx, "hello world")
  - Call EmbedQuery(ctx, "hello world")
Assert:
  - Document request body has input = "search_document: hello world"
  - Query request body has input = "search_query: hello world"
  - Both request bodies have model = "nomic-embed-text" (default)
  - Both return []float32{0.1, 0.2, 0.3}
```

#### Test 2: Successful embedding round-trip

```
Setup: httptest server returns {"embeddings": [[0.1, 0.2, 0.3]]}
Test: Call EmbedQuery(ctx, "test")
Assert: Returns []float32{0.1, 0.2, 0.3}, nil error
```

#### Test 3: Retry on 5xx

```
Setup: httptest server returns 500 twice (with body "server overloaded"),
       then 200 with {"embeddings": [[0.1, 0.2, 0.3]]}
       Use atomic counter to track request count.
Test: Call EmbedQuery(ctx, "test")
       Override localInitialDelay if possible, or accept slow test (~6s).
Assert:
  - Server received 3 requests (2 failures + 1 success)
  - Returns []float32{0.1, 0.2, 0.3}
```

**Note on timing:** The retry loop uses `time.Duration(math.Pow(2, float64(attempt))) * localInitialDelay`. Attempts 1 and 2 sleep 2s and 4s. To keep tests fast, either:
- Accept ~6s test duration (acceptable for 1 test), OR
- Use a short-lived context with generous timeout

#### Test 4: No retry on 4xx

```
Setup: httptest server always returns 400 with body "bad request"
Test: Call EmbedQuery(ctx, "test")
Assert:
  - Server received exactly 1 request
  - Error returned immediately
  - Error contains "local embedding error (400)"
```

#### Test 5: Max retries exhausted

```
Setup: httptest server always returns 500
Test: Call EmbedQuery(ctx, "test")
Assert:
  - Server received exactly 5 requests (localMaxRetries)
  - Error contains "max retries (5) exceeded"
```

**Note on timing:** This test sleeps 2+4+8+16 = 30s total. Consider using a short context timeout to bail early once the test proves the retry count, or accept the duration for CI.

#### Test 6: Context cancellation during retry

```
Setup:
  - httptest server always returns 500
  - Create context with cancel
  - Cancel context after first response (use server handler to signal)
Test: Call EmbedQuery with cancellable context
Assert:
  - Returns context.Canceled error
  - Server received 1-2 requests (not all 5)
```

#### Test 7: Empty embeddings response

```
Setup: httptest server returns {"embeddings": []}
Test: Call EmbedQuery(ctx, "test")
Assert: Error contains "no embeddings returned"
```

#### Test 8: Custom URL and model

```
Setup: httptest server that checks request body for model field
Test: NewLocalClient(WithLocalBaseURL(server.URL), WithLocalModel("custom-model"))
      Call EmbedQuery(ctx, "test")
Assert:
  - Request sent to server.URL (not localhost:11434)
  - Request body has model = "custom-model"
  - Returns valid embedding
```

### Estimated size

~250 lines. 8 test functions. No external dependencies (httptest only).

---

## Gap 3: Add→Search Round-Trip (`codex/eval/roundtrip_test.go`)

> **Status: IMPLEMENTED** — `roundtrip_test.go` was added in commit `71df634` with all 5 subtests described below.

### What it proves

RECALL's core value loop: knowledge added in one operation is retrievable by search in a later operation.

### Setup pattern

Follow `harness_test.go`: one expensive setup, many cheap subtests. The key difference from the harness is that we add items via the MCP protocol (not bulk indexing), then search via the MCP protocol.

```go
//go:build fts5 && evalintegration

func TestRoundTrip(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    t.Cleanup(cancel)

    // Create real SearchEngine with real SQLite + real Ollama
    dbPath := filepath.Join(t.TempDir(), "roundtrip.db")
    engine, err := core.NewSearchEngine(ctx, core.Config{
        MetadataDBPath: dbPath,
    })
    if err != nil {
        t.Fatalf("NewSearchEngine: %v", err)
    }
    t.Cleanup(func() { engine.Close() })

    // Boot MCP server via io.Pipe (in-process, no subprocess)
    clientR, serverW := io.Pipe()
    serverR, clientW := io.Pipe()
    server := mcp.NewServer(engine, "roundtrip-test")
    done := make(chan error, 1)
    go func() { done <- server.RunForIO(ctx, serverR, serverW) }()
    t.Cleanup(func() { clientW.Close(); <-done })

    // Use a simple JSON-RPC helper to send/receive
    // (or adapt edi/internal/testutil/MCPTestClient to work with io.Pipe)
    ...

    // Initialize protocol once
    // ... send initialize, notifications/initialized ...

    t.Run("AddThenSearch", ...)
    t.Run("SemanticMatch", ...)
    t.Run("ScopeIsolation", ...)
    t.Run("FlightRecorderAudit", ...)
    t.Run("FeedbackRoundTrip", ...)
}
```

**Important:** `engine.Add()` is synchronous — it embeds and stores before returning. No delay needed between add and search.

### What to test

**File: `codex/eval/roundtrip_test.go`**

Build tag: `fts5 evalintegration` (requires Ollama like the existing PayFlow harness).

#### Test 1: Add then search — item is found

```
Test:
  1. recall_add: type="failure", title="Payment retry without jitter causes
     thundering herd", content="When multiple payment processors retry
     simultaneously without jitter, they overwhelm the downstream service.
     Fix: add random jitter to exponential backoff delays.",
     tags=["retry", "jitter", "thundering-herd"]
  2. Note the returned ID from the add response
  3. recall_search: query="how to avoid thundering herd when retrying payments"

Assert:
  - Search returns ≥1 result
  - Top result's ID matches the item we just added
  - Top result's title contains "jitter" or "thundering herd"
```

#### Test 2: Add then search — semantic match (not just keyword)

```
Test:
  1. recall_add: type="pattern", title="Circuit breaker state machine",
     content="Use three states: closed (normal), open (failing), half-open
     (testing). Transition open→half-open after a timeout. Allow one request
     through in half-open to test recovery."

  2. recall_search: query="how to handle cascading failures in microservices"
     (no keyword overlap with the stored content)

Assert:
  - Search returns the circuit breaker item (semantic similarity)
  - This proves vector search works, not just FTS5 keyword matching
```

#### Test 3: Scope isolation

```
Test:
  1. recall_add with scope="project-a", content about database migrations
  2. recall_add with scope="project-b", content about API versioning
  3. recall_search with scope="project-a", query="database"

Assert:
  - Returns the database item (project-a)
  - Does NOT return the API item (project-b)
```

#### Test 4: Flight recorder audit trail

```
Test:
  1. recall_add an item
  2. recall_search with a query
  3. Query flight recorder entries via engine.GetFlightRecorderEntries("roundtrip-test")

Assert:
  - Flight recorder contains a "retrieval_query" entry for the search
  - Entry metadata includes the query text and result count
```

#### Test 5: Feedback round-trip

```
Test:
  1. recall_add an item, note the ID from the response
  2. recall_feedback with item_id=<id>, useful=true, context="helped avoid bug"
  3. Verify no error returned

Assert:
  - No error on feedback call
  - Response has status="recorded"
```

### Why this test matters

The PayFlow harness tests retrieval quality against a static corpus of 30 pre-defined documents. This test proves the *dynamic* loop: items added via the MCP protocol are findable via the MCP protocol. It's the difference between "the index works" and "the tool works end-to-end."

### Estimated size

~300 lines. 5 test functions. Requires Ollama (same as existing PayFlow harness).

---

## Gap 4: Hook Configuration Validation

### What hooks actually are in this codebase

After auditing the code, hooks in AEF exist at two levels:

1. **EDI session context**: `edi/internal/launch/context.go` builds a system prompt that tells Claude about hooks. But these are *instructions in prose*, not executable hook configurations.

2. **Eval condition config**: `codex/eval/condition.go` defines `HookConfig` structs (e.g., `PostToolUse:Write:*.go → gofumpt -w $FILE`). But these are eval-level abstractions that would be written to `.claude/settings.json` — they don't execute in the eval runners.

3. **Claude Code hooks**: Actual `.claude/settings.json` hook entries that Claude Code executes. **These don't exist in the repo.**

4. **Task sync hook**: `edi/cmd/task-sync-hook/main.go` — a real binary that Claude Code calls as a hook. This IS tested via `tasks/sync_test.go`.

### What to test

The task sync hook is the only real, executable hook. The gofumpt/protect-files hooks are aspirational config that would go in `.claude/settings.json` but aren't deployed.

**File: `edi/internal/launch/context_test.go`** (extend existing test file)

#### Test 1: Context includes hook instructions

```
Test: Call BuildContext with a config that has hooks enabled
Assert:
  - Context output contains "gofumpt" (the formatter hook mentioned in prose)
  - Context output contains "PostToolUse" or the hook pattern format
  - Context output is valid (no broken template rendering — no "{{" or "<no value>" artifacts)
```

#### Test 2: Task sync hook binary runs

```
(This already exists in tasks/sync_test.go — verify it passes)
```

#### Test 3: Condition hook config generates valid entries

```
(This already exists in condition_test.go — verify the HookConfig
 structs produce correct Matcher and Command fields)
```

### What NOT to build

Don't build a hook execution test framework. Hooks are executed by Claude Code's runtime, not by AEF. Testing whether Claude Code correctly executes `.claude/settings.json` hooks is Claude Code's responsibility. AEF's responsibility is:
- Generating the correct hook configuration (tested by condition_test.go)
- Providing a working hook binary (tested by tasks/sync_test.go)
- Documenting hooks in the system prompt (tested by context_test.go)

### Estimated size

~50 lines of additions to existing test files. No new test files needed.

---

## Gap 5: Chunking (`codex/internal/chunking/`)

### What it does

The chunking package splits source code and documents into semantically meaningful pieces for embedding and indexing. It has three components:

1. **AST chunker** (`ast.go`, 447 lines): Uses tree-sitter (pure Go, no CGO) to parse Go, Python, and TypeScript/JavaScript into function/method/class/type chunks. Falls back to 100-line overlapping chunks when AST parsing fails or the language is unsupported.

2. **Markdown chunker** (`markdown.go`, 140 lines): Splits markdown by headers into sections, with optional further splitting of large sections by paragraph boundaries.

3. **Contextual chunker** (`contextual.go`, 94 lines): Stub — `EnrichChunk` always returns "not implemented". `ChunkDocument` works but without enrichment (graceful degradation). **Skip testing** — there's nothing to verify beyond the error message.

### What to test

**File: `codex/internal/chunking/ast_test.go`**

**No build tags needed** — tree-sitter is pure Go (via `github.com/smacker/go-tree-sitter`), no CGO or FTS5.

#### Test 1: Go function extraction

```
Setup: Source code string:
  package main

  import "fmt"

  func Hello(name string) string {
      return fmt.Sprintf("hello %s", name)
  }

  func Add(a, b int) int {
      return a + b
  }

Test: chunker.ChunkFile([]byte(source), "go", "main.go")
Assert:
  - Returns 2 chunks
  - chunks[0].Name == "Hello", chunks[0].Type == "function"
  - chunks[1].Name == "Add", chunks[1].Type == "function"
  - Each chunk's Content contains the full function body
  - Each chunk's Language == "go"
  - Each chunk's FilePath == "main.go"
  - StartLine and EndLine are correct (1-indexed)
  - Signature contains "func Hello(name string) string"
```

#### Test 2: Go method and type extraction

```
Setup: Source code with struct type and method:
  package main

  type Server struct {
      port int
      host string
  }

  func (s *Server) Start() error {
      return nil
  }

Test: chunker.ChunkFile([]byte(source), "go", "server.go")
Assert:
  - Returns 2 chunks: one type, one method
  - Type chunk: Name == "Server" (extracted from first "identifier" child), Type == "type"
  - Method chunk: Name == "Start", Type == "method"
  - Type chunk content includes full struct definition
```

#### Test 3: Python function and class extraction

```
Setup: Python source:
  def greet(name):
      return f"hello {name}"

  class Calculator:
      def __init__(self):
          self.result = 0

      def add(self, x):
          self.result += x
          return self

Test: chunker.ChunkFile([]byte(source), "python", "calc.py")
Assert:
  - Returns 2 chunks (greet function + Calculator class)
  - chunks[0].Name == "greet", Type == "function"
  - chunks[1].Name == "Calculator", Type == "class"
  - Class chunk includes the entire class body (methods included)
  - Language == "python" for both
```

#### Test 4: TypeScript extraction

```
Setup: TypeScript source:
  interface Config {
      port: number;
      host: string;
  }

  function createServer(config: Config): void {
      console.log(config.port);
  }

  class App {
      start(): void {}
  }

Test: chunker.ChunkFile([]byte(source), "typescript", "app.ts")
Assert:
  - Returns 3 chunks: interface (type), function, class
  - Interface chunk: Name == "Config", Type == "type"
  - Function chunk: Name == "createServer", Type == "function"
  - Class chunk: Name == "App", Type == "class"
```

#### Test 5: JavaScript and JSX/TSX language aliases

```
Tests (table-driven):
  | Language     | FilePath    | Parsed by TS parser |
  |-------------|-------------|---------------------|
  | "typescript" | "a.ts"     | yes                 |
  | "tsx"        | "a.tsx"    | yes                 |
  | "javascript" | "a.js"    | yes                 |
  | "jsx"        | "a.jsx"   | yes                 |

Setup: Simple function source for each
Assert: All produce at least 1 chunk (proves TS parser handles all 4 aliases)
```

#### Test 6: Fallback to line-based chunking

```
Tests:
  a) Unsupported language:
     - chunker.ChunkFile([]byte(rustCode), "rust", "main.rs")
     - Returns chunks with Type == "chunk", Name == ""
     - Chunk size ~100 lines with 10-line overlap

  b) Empty content:
     - chunker.ChunkFile([]byte(""), "go", "empty.go")
     - Returns 0 chunks (empty content after trim produces nothing)

  c) Content with no extractable nodes (e.g., only comments):
     - chunker.ChunkFile([]byte("// just a comment\n"), "go", "comment.go")
     - Falls back to line-based chunks (no AST nodes extracted)
```

#### Test 7: Fallback chunk sizing

```
Setup: Generate a 250-line file (e.g., numbered lines "line 1\nline 2\n...")
Test: chunker.ChunkFile(content, "rust", "big.rs")  // unsupported → fallback
Assert:
  - Returns 3 chunks:
    chunk[0]: StartLine=1, EndLine=100  (lines 1-100)
    chunk[1]: StartLine=91, EndLine=190 (lines 91-190, 10-line overlap)
    chunk[2]: StartLine=181, EndLine=250 (lines 181-250)
  - Each chunk's Content contains the correct lines
  - Overlap: last 10 lines of chunk[0] == first 10 lines of chunk[1]
```

#### Test 8: DetectLanguage

```
Tests (table-driven):
  | FilePath           | Expected Language |
  |--------------------|-------------------|
  | "main.go"          | "go"              |
  | "script.py"        | "python"          |
  | "app.ts"           | "typescript"      |
  | "component.tsx"    | "tsx"             |
  | "index.js"         | "javascript"      |
  | "App.jsx"          | "jsx"             |
  | "main.rs"          | "rust"            |
  | "Main.java"        | "java"            |
  | "util.c"           | "c"               |
  | "util.h"           | "c"               |
  | "util.cpp"         | "cpp"             |
  | "util.hpp"         | "cpp"             |
  | "Makefile"         | "unknown"         |
  | "README.md"        | "unknown"         |
  | "UPPER.GO"         | "go"              |  ← case insensitive

Assert: DetectLanguage(path) == expected for each row
```

#### Test 9: GetSupportedLanguages and IsAvailable

```
Test:
  chunker := NewASTChunker()
  assert chunker.IsAvailable() == true
  assert chunker.GetSupportedLanguages() == ["go", "python", "typescript", "tsx", "javascript", "jsx"]
  assert chunker.Close() == nil
```

---

**File: `codex/internal/chunking/markdown_test.go`**

#### Test 10: SplitMarkdown basic header splitting

```
Setup: Markdown with 3 sections:
  # Introduction
  Some intro text.

  ## Methods
  Method details here.
  More method text.

  ## Results
  Results text.

Test: SplitMarkdown(content)
Assert:
  - Returns 3 sections
  - sections[0]: Title="Introduction", Level=1, Content contains "Some intro text."
  - sections[1]: Title="Methods", Level=2, Content contains "Method details" + "More method text."
  - sections[2]: Title="Results", Level=2, Content contains "Results text."
  - StartLine/EndLine are correct for each section
```

#### Test 11: SplitMarkdown edge cases

```
Tests:
  a) Content before first header → "(Introduction)" section with Level=0:
     Input: "Some text\n\n# First Header\nContent"
     Assert: sections[0].Title == "(Introduction)", sections[0].Level == 0

  b) No headers at all → single "(Document)" section:
     Input: "Just plain text\nwith multiple lines"
     Assert: len(sections) == 1, sections[0].Title == "(Document)", Level == 0

  c) Empty string → empty slice:
     Input: ""
     Assert: len(sections) == 0

  d) Only whitespace → empty slice:
     Input: "   \n\n   "
     Assert: len(sections) == 0

  e) Header with no content after it (at end of file) → section skipped (empty content):
     Input: "# Title\nContent\n## Empty"
     Assert: len(sections) == 1 (the "Empty" section has no content, gets skipped)

  f) Consecutive headers → only headers with content produce sections:
     Input: "# A\n## B\nContent of B"
     Assert: sections produced for "B" (with content), "A" skipped (no content)
```

#### Test 12: ChunkMarkdown splits large sections

```
Setup: Markdown where one section has 5000 bytes of content (multiple paragraphs
       separated by \n\n), maxChunkSize = 1000
Test: ChunkMarkdown(content, 1000)
Assert:
  - Small sections pass through unchanged
  - Large section is split into multiple MarkdownSection entries
  - Each sub-section has Title == original section's Title
  - Each sub-section's Content length ≤ 1000 bytes (approximately — paragraph boundaries may cause slight excess)
  - All original content is preserved across the sub-sections
```

#### Test 13: ChunkMarkdown with maxChunkSize <= 0

```
Test: ChunkMarkdown(content, 0)
Assert: Returns same result as SplitMarkdown(content) — no further splitting
```

#### Test 14: splitByParagraphs

```
Tests:
  a) Content with 3 small paragraphs, maxSize = 1000:
     Input: "para1\n\npara2\n\npara3"
     Assert: Returns 1 chunk (all fit)

  b) Content with 3 paragraphs, maxSize = 20:
     Input: "short paragraph one\n\nshort paragraph two\n\nshort paragraph three"
     Assert: Returns 3 chunks (each paragraph alone)

  c) Single paragraph larger than maxSize:
     Input: "a very long paragraph that exceeds maxSize..."
     Assert: Returns 1 chunk (can't split further)
```

### Estimated size

~400 lines across 2 test files. 14 test functions. No external dependencies (tree-sitter is pure Go).

---

## Summary

| Gap | New File | Tests | Lines | Dependencies | What It Proves |
|---|---|---|---|---|---|
| MCP server | `codex/internal/mcp/server_test.go` | 8 | ~450 | None (io.Pipe + mock) | Claude Code can talk to RECALL correctly |
| Embedding client | `codex/internal/embedding/local_test.go` | 8 | ~250 | None (httptest) | Embeddings generate correctly, retries work |
| ~~Add→search round-trip~~ | `codex/eval/roundtrip_test.go` | 5 | ~300 | Ollama | **DONE** — Knowledge added is knowledge found |
| Hook config | Extend existing files | 1-2 | ~50 | None | Hook instructions are correct |
| Chunking | `codex/internal/chunking/ast_test.go` + `markdown_test.go` | 14 | ~400 | None (tree-sitter is pure Go) | Code and docs split into correct, meaningful chunks |
| **Total** | **4 new files** | **~37** | **~1,450** | | |

### What to run

```bash
# Gap 1+4: MCP server + hook config (no external deps)
cd codex && go test -race -tags "fts5" ./internal/mcp/...
cd edi && go test -race -tags "fts5" ./internal/launch/...

# Gap 2: Embedding client (no external deps)
cd codex && go test -race ./internal/embedding/...

# Gap 3: Round-trip (requires Ollama)
cd codex && go test -race -tags "fts5 evalintegration" ./eval/... -run TestRoundTrip -timeout 10m

# Gap 5: Chunking (no external deps, tree-sitter is pure Go)
cd codex && go test -race ./internal/chunking/...
```
