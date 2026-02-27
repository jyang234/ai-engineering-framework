# Test Spec: Filling the Gaps

> Date: 2026-02-27
> Question: Are the tools built well and built in a way that allows Claude to use them effectively?
> Scope: 4 untested areas identified in the audit

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

### What to test

**File: `codex/internal/mcp/server_test.go`**

Use `RunForIO` with `io.Pipe` — no subprocess, no Ollama, fast.

#### Test 1: Protocol lifecycle

```
Setup: Create Server with mock SearchEngine, connect via io.Pipe
Test:
  1. Send initialize → expect protocolVersion, serverInfo.name == "recall-mcp"
  2. Send notifications/initialized → no response (notification)
  3. Send tools/list → expect 5 tools: recall_search, recall_get, recall_add,
     recall_feedback, flight_recorder_log
  4. Each tool has name, description, inputSchema with required fields
Assert: Protocol completes without error, tool count and names correct
```

#### Test 2: recall_search happy path

```
Setup: Mock engine returns 3 items with scores [0.9, 0.7, 0.5]
Test: Send tools/call recall_search with query "payment retry"
Assert:
  - Result has 3 items, sorted by score descending
  - Each item has: rank, id, type, title, content, tags, score, score_pct
  - score_pct computed correctly (top = 100%, second = 78%, third = 56%)
  - Flight recorder auto-logged a retrieval_query entry (verify mock called)
```

#### Test 3: recall_search parameter validation

```
Tests (table-driven):
  - Empty query → isError: true, "query is required"
  - Query > 10KB → isError: true, "exceeds maximum size"
  - Valid query with types filter → engine.Search called with types
  - Valid query with scope filter → engine.Search called with scope
  - Valid query with limit=5 → engine.Search called with limit 5
  - No limit specified → default limit 10
```

#### Test 4: recall_add happy path + env var injection

```
Setup: Set EDI_SESSION_ID, EDI_GIT_BRANCH, EDI_GIT_SHA env vars
Test: Send recall_add with type="pattern", title="retry", content="..."
Assert:
  - Engine.Add called with correct type, title, content
  - Metadata includes session_id, git_branch, git_sha from env vars
  - Response has id and success message
Cleanup: Unset env vars
```

#### Test 5: recall_add parameter validation

```
Tests (table-driven):
  - Missing type → isError: true
  - Missing title → isError: true
  - Missing content → isError: true
  - Content > 1MB → isError: true, "exceeds maximum size"
  - Default scope → "project"
  - Explicit scope → passed through
```

#### Test 6: recall_get, recall_feedback, flight_recorder_log

```
recall_get:
  - Valid ID → engine.Get called, item returned
  - Missing ID → isError: true

recall_feedback:
  - Valid item_id + useful=true → engine.RecordFeedback called
  - Missing item_id → isError: true
  - Missing useful → isError: true
  - useful is string not bool → isError: true, "must be a boolean"

flight_recorder_log:
  - Valid type + content → engine.LogFlightRecorder called
  - Missing type → isError: true
  - Missing content → isError: true
  - Optional rationale + metadata → passed through
```

#### Test 7: JSON-RPC error codes

```
Tests:
  - Invalid JSON → error code -32700
  - Unknown method "foo/bar" → error code -32601
  - Invalid params (string instead of object) → error code -32602
  - Unknown tool name → isError: true in result (not protocol error)
```

#### Test 8: Engine error propagation

```
Setup: Mock engine returns error on Search, Add, Get
Test: Call each tool
Assert: isError: true, error message contains engine error text
```

### Mock strategy

Create a `mockSearchEngine` struct satisfying the same interface the real `SearchEngine` uses. Record calls and return configurable results. The V0 RECALL tests in `edi/internal/recall/` show a similar pattern.

### Estimated size

~400 lines. 8 test functions. No external dependencies.

---

## Gap 2: Embedding Client (`codex/internal/embedding/`)

### What it does

`LocalClient` calls an Ollama-compatible HTTP endpoint to generate vector embeddings. It prefixes queries with "search_query: " and documents with "search_document: " for asymmetric retrieval. It retries on 5xx errors with exponential backoff.

### What to test

**File: `codex/internal/embedding/local_test.go`**

Use `httptest.NewServer` to mock Ollama — no real Ollama needed.

#### Test 1: EmbedDocument and EmbedQuery prefixes

```
Setup: httptest server that records request body, returns valid embedding
Test:
  - Call EmbedDocument("hello world")
  - Call EmbedQuery("hello world")
Assert:
  - Document request input = "search_document: hello world"
  - Query request input = "search_query: hello world"
  - Both return the embedding from the mock response
```

#### Test 2: Successful embedding round-trip

```
Setup: httptest server returns {"embeddings": [[0.1, 0.2, 0.3]]}
Test: Call EmbedQuery("test")
Assert: Returns []float32{0.1, 0.2, 0.3}
```

#### Test 3: Retry on 5xx

```
Setup: httptest server returns 500 twice, then 200 with valid embedding
Test: Call EmbedQuery("test")
Assert:
  - Server received 3 requests (2 retries + 1 success)
  - Returns valid embedding
```

#### Test 4: No retry on 4xx

```
Setup: httptest server returns 400
Test: Call EmbedQuery("test")
Assert:
  - Server received exactly 1 request
  - Error returned immediately (no retry)
```

#### Test 5: Max retries exhausted

```
Setup: httptest server always returns 500
Test: Call EmbedQuery("test")
Assert:
  - Server received 5 requests (localMaxRetries)
  - Error contains "max retries (5) exceeded"
```

#### Test 6: Context cancellation during retry

```
Setup: httptest server returns 500; cancel context after first attempt
Test: Call EmbedQuery with cancelled context
Assert: Returns context.Canceled error, doesn't retry after cancellation
```

#### Test 7: Empty embeddings response

```
Setup: httptest server returns {"embeddings": []}
Test: Call EmbedQuery("test")
Assert: Error "no embeddings returned"
```

#### Test 8: Custom URL and model

```
Setup: httptest server that checks request body for model field
Test: NewLocalClient(WithLocalBaseURL(server.URL), WithLocalModel("custom-model"))
Assert: Request sent to custom URL with model "custom-model"
```

### Estimated size

~250 lines. 8 test functions. No external dependencies (httptest only).

---

## Gap 3: Add→Search Round-Trip (`codex/eval/roundtrip_test.go`)

### What it proves

RECALL's core value loop: knowledge added in one operation is retrievable by search in a later operation. This is the single most important integration test missing from the codebase.

### What to test

**File: `codex/eval/roundtrip_test.go`**

Build tag: `fts5 evalintegration` (requires Ollama like the existing PayFlow harness).

#### Test 1: Add then search — item is found

```
Setup:
  - Create real SearchEngine with real SQLite, real Ollama embeddings
  - Boot MCP server via RunForIO (io.Pipe, in-process)
  - Initialize protocol

Test:
  1. recall_add: type="failure", title="Payment retry without jitter causes
     thundering herd", content="When multiple payment processors retry
     simultaneously without jitter, they overwhelm the downstream service.
     Fix: add random jitter to exponential backoff delays.",
     tags=["retry", "jitter", "thundering-herd"]

  2. recall_search: query="how to avoid thundering herd when retrying payments"

Assert:
  - Search returns ≥1 result
  - Top result's ID matches the item we just added
  - Top result's title contains "jitter" or "thundering herd"
```

#### Test 2: Add then search — semantic match (not just keyword)

```
Setup: Same as Test 1

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
  3. Query flight recorder entries for the session

Assert:
  - Flight recorder contains a retrieval_query entry for the search
  - Entry includes the query text and result count
```

#### Test 5: Feedback round-trip

```
Test:
  1. recall_add an item, note the ID
  2. recall_feedback with item_id=<id>, useful=true, context="helped avoid bug"
  3. Verify feedback was recorded (query engine directly or via flight recorder)

Assert:
  - No error on feedback
  - Feedback associated with correct item
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

**File: `edi/internal/launch/context_test.go`** (extend existing)

#### Test 1: Context includes hook instructions

```
Test: Call BuildContext with a config that has hooks enabled
Assert:
  - Context file mentions gofumpt
  - Context file mentions the PostToolUse hook pattern
  - Context file is valid markdown (no broken formatting)
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

## Summary

| Gap | New File | Tests | Lines | Dependencies | What It Proves |
|---|---|---|---|---|---|
| MCP server | `codex/internal/mcp/server_test.go` | 8 | ~400 | None (io.Pipe + mock) | Claude Code can talk to RECALL correctly |
| Embedding client | `codex/internal/embedding/local_test.go` | 8 | ~250 | None (httptest) | Embeddings generate correctly, retries work |
| Add→search round-trip | `codex/eval/roundtrip_test.go` | 5 | ~300 | Ollama | Knowledge added is knowledge found |
| Hook config | Extend existing files | 1-2 | ~50 | None | Hook instructions are correct |
| **Total** | **3 new files** | **~23** | **~1,000** | | |

Compare to the L3 eval corpus: **~1,000 lines proving the tools work** vs **~13,000 lines testing whether prompting helps**.

### What to run

```bash
# Gap 1+4: MCP server + hook config (no external deps)
cd codex && go test -race -tags "fts5" ./internal/mcp/...
cd edi && go test -race -tags "fts5" ./internal/launch/...

# Gap 2: Embedding client (no external deps)
cd codex && go test -race ./internal/embedding/...

# Gap 3: Round-trip (requires Ollama)
cd codex && go test -race -tags "fts5 evalintegration" ./eval/... -run TestRoundTrip -timeout 10m
```
