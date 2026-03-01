# ADR: Codex Production Hardening for Small-Team Use

| Field   | Value                                                              |
|---------|--------------------------------------------------------------------|
| Status  | Proposed                                                           |
| Date    | 2026-03-01                                                         |
| Authors | AI Engineering Framework Team                                      |
| Scope   | `codex/internal/core/`, `codex/internal/mcp/`, `codex/internal/embedding/`, `codex/cmd/recall-mcp/`, `edi/internal/launch/` |

## Context

Codex v1 is architecturally sound and functionally complete for its intended
scope. The hybrid search engine (vector + FTS5 + RRF), MCP server, and eval
infrastructure all work correctly under ideal conditions. However, daily use by
a small engineering team (2-5 developers) exposes six categories of failure that
the current implementation does not handle gracefully:

1. **Total search outage when Ollama is unavailable.** The `Search()` pipeline
   calls `EmbedQuery()` first (engine.go:125). If embedding fails, the entire
   search returns an error. There is no fallback to keyword-only FTS5 search.
   Ollama will go down — laptop restarts, OOM kills, Docker hiccups. When it
   does, every `recall_search` call fails completely.

2. **Silent hang when MCP server fails to start.** EDI writes `.mcp.json` and
   execs Claude Code. Claude Code starts the MCP server as a subprocess via
   stdio. If `recall-mcp` can't initialize (bad DB path, corrupt schema, Ollama
   unreachable for reranker), it calls `log.Fatalf` and exits. Claude Code is
   already running and hangs waiting for MCP responses that will never arrive.
   There is no readiness check.

3. **No operational visibility.** The MCP server communicates errors back via
   JSON-RPC protocol but never logs anything to stderr or a file. When a
   developer says "search isn't returning good results" or "it felt slow today,"
   there is nothing to look at. No request timing, no error counts, no query
   traces.

4. **Embedding timeouts can freeze Claude Code for 2+ minutes.** The HTTP
   client timeout is 30s (local.go:51). With 5 retries and exponential backoff
   (1s + 2s + 4s + 8s + 16s = 31s), a single search against a struggling Ollama
   instance blocks for over 2 minutes. During that time, Claude Code is frozen
   waiting for the MCP response.

5. **Stale vector cache across concurrent sessions.** `VecStore.loadAll()`
   loads all vectors into memory at startup (vecstore.go:41). If two developers
   run overlapping sessions against the same codex DB, process A won't see
   documents that process B added until A restarts. FTS5 stays in sync (SQLite
   handles it), but vector search returns stale results silently.

6. **Eval thresholds are decorative.** The E2E test (harness_test.go:79-84)
   requires Recall@10 >= 0.3 and MRR >= 0.2. Actual benchmark results show
   Recall@10 at 0.908 and MRR at 0.842. Search quality could degrade by 66% and
   CI would still pass.

None of these require architectural changes. They are wiring in failure modes
that the current implementation is missing.

## Decision

Implement six targeted hardening changes, organized as three tiers: must-have
(blocks daily use), should-have (blocks team confidence), and improve (blocks
quality assurance).

---

## Change 1: FTS5 Keyword Fallback When Embedding Fails

**Tier:** Must-have
**Files:** `codex/internal/core/engine.go`
**Estimated effort:** ~2 hours

### Current Behavior

```
Search() → EmbedQuery() fails → return nil, fmt.Errorf("failed to embed query: %w", err)
```

The entire search pipeline aborts. No results returned. Claude Code shows an
error to the user.

### Proposed Behavior

```
Search() → EmbedQuery() fails → log warning → skip vector search → run FTS5 only → return keyword results
```

When embedding fails, the search degrades to keyword-only mode. Results are
returned with a `degraded: true` flag so the caller can inform the user.

### Specification

#### 1.1 Modify `SearchEngine.Search()` (engine.go:111-225)

Replace the hard failure at line 125-128 with graceful degradation:

```go
// 1. Embed query (graceful — fall back to keyword-only if embedding unavailable)
var vectorResults []storage.ScoredResult
queryVec, embedErr := e.embedder.EmbedQuery(ctx, req.Query)
if embedErr != nil {
    log.Printf("Warning: embedding unavailable, falling back to keyword-only search: %v", embedErr)
} else {
    // 2. Vector search
    vectorResults, err = e.vecStore.Search(ctx, queryVec, candidateLimit)
    if err != nil {
        log.Printf("Warning: vector search failed: %v", err)
    }
}
```

The rest of the pipeline (keyword search, RRF fusion, filtering, reranking,
threshold, limit) remains unchanged. When `vectorResults` is nil, RRF fusion
returns keyword-only results — this already works because `reciprocalRankFusion`
handles empty first-source gracefully.

#### 1.2 Add `Degraded` field to `SearchResult`

In `codex/internal/core/types.go`, add to the SearchResult struct:

```go
type SearchResult struct {
    Item
    Score    float64
    Degraded bool // true when results are keyword-only due to embedding failure
}
```

Set `Degraded: true` on all results when `embedErr != nil`.

#### 1.3 Surface degradation in MCP response

In `codex/internal/mcp/tools.go` `handleSearch()`, propagate the flag:

```go
response := map[string]interface{}{
    "results": ranked,
    "count":   len(ranked),
}
if anyDegraded(results) {
    response["warning"] = "Results are keyword-only: embedding service unavailable"
}
```

#### 1.4 Verify RRF handles empty vector source

Confirm that `reciprocalRankFusion(nil, keywordResults, 60)` returns
`keywordResults` correctly scored. If not, add a nil guard:

```go
func reciprocalRankFusion(vecResults []storage.ScoredResult, kwResults []SearchResult, k int) []SearchResult {
    if len(vecResults) == 0 && len(kwResults) == 0 {
        return nil
    }
    // ... existing implementation
}
```

### Testing

- **Unit test (engine_test.go):** Add `TestSearch_EmbeddingFallback` — inject a
  `MockEmbedder` that returns an error. Verify keyword results are still
  returned. Verify `Degraded == true` on all results.
- **Unit test (engine_test.go):** Add `TestSearch_VectorSearchFallback` —
  embedder succeeds but `MockVectorStorage.Search()` fails. Verify keyword
  results returned.
- **MCP test (server_test.go):** Add `TestRecallSearchDegradedMode` — stub
  embedder that fails. Verify response includes `warning` field and results are
  non-empty.
- **Eval harness:** No changes needed — eval already requires Ollama.

### Risks

- **Keyword-only results may be poor quality for semantic queries.** This is
  strictly better than returning zero results.
- **Users may not notice the degradation warning.** The MCP response includes it
  in the `_judge_reminder` context, so Claude Code will see it. Consider logging
  the warning to the flight recorder as well.

---

## Change 2: MCP Server Startup Health Check

**Tier:** Must-have
**Files:** `codex/cmd/recall-mcp/main.go`, `codex/internal/core/engine.go`, `codex/internal/embedding/local.go`
**Estimated effort:** ~2 hours

### Current Behavior

```
recall-mcp main() → NewSearchEngine() → on failure: log.Fatalf()
```

`NewSearchEngine()` (engine.go:36-79) opens the DB, loads vectors, creates the
embedding client, and optionally loads the reranker. The embedding client is
constructed but **never tested** — `NewLocalClient()` just saves the URL and
returns. If Ollama is unreachable, the server starts successfully and only fails
on the first `recall_search` call.

The DB open can fail (corrupt file, permissions), which `log.Fatalf` handles.
But the common failure mode — Ollama not running — is invisible until the first
search.

### Proposed Behavior

```
recall-mcp main() → NewSearchEngine()
                   → engine.HealthCheck() → ping Ollama, verify DB reads
                   → on failure: log warning to stderr, continue (degraded)
                   → on success: log "ready" to stderr
```

The health check is **informational, not blocking**. The server starts even if
Ollama is down (because Change 1 provides FTS5 fallback). But it logs clearly
what is and isn't working.

### Specification

#### 2.1 Add `Embedder.Ping()` method

In `codex/internal/core/interfaces.go`:

```go
type Embedder interface {
    EmbedDocument(ctx context.Context, text string) ([]float32, error)
    EmbedQuery(ctx context.Context, query string) ([]float32, error)
    // Ping verifies the embedding service is reachable.
    // Returns nil if healthy, error with details if not.
    Ping(ctx context.Context) error
}
```

In `codex/internal/embedding/local.go`:

```go
// Ping sends a minimal embedding request to verify Ollama is reachable.
func (c *LocalClient) Ping(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    _, err := c.embed(ctx, "ping")
    if err != nil {
        return fmt.Errorf("embedding service unreachable at %s: %w", c.baseURL, err)
    }
    return nil
}
```

Use a short 5-second timeout rather than the full 30-second client timeout.
This is a startup probe, not a production request.

#### 2.2 Add `SearchEngine.HealthCheck()` method

In `codex/internal/core/engine.go`:

```go
// HealthStatus reports the operational state of the search engine.
type HealthStatus struct {
    DBHealthy        bool
    EmbeddingHealthy bool
    VectorCount      int
    ItemCount        int
    EmbeddingError   string // empty if healthy
}

// HealthCheck probes all dependencies and returns their status.
// It does not fail — it reports what is and isn't working.
func (e *SearchEngine) HealthCheck(ctx context.Context) HealthStatus {
    status := HealthStatus{}

    // Check DB
    if counts, err := e.metadata.CountItemsByType(); err == nil {
        status.DBHealthy = true
        for _, c := range counts {
            status.ItemCount += c
        }
    }

    // Check vector store
    if vs, ok := e.vecStore.(interface{ Count() int }); ok {
        status.VectorCount = vs.Count()
    }

    // Check embedding service
    if err := e.embedder.Ping(ctx); err != nil {
        status.EmbeddingError = err.Error()
    } else {
        status.EmbeddingHealthy = true
    }

    return status
}
```

#### 2.3 Call health check on startup

In `codex/cmd/recall-mcp/main.go`, after `NewSearchEngine()` succeeds:

```go
engine, err := core.NewSearchEngine(ctx, config)
if err != nil {
    log.Fatalf("Failed to initialize search engine: %v", err)
}
defer engine.Close()

// Startup health check — informational, not blocking
status := engine.HealthCheck(ctx)
log.Printf("Health check: db=%v embedding=%v vectors=%d items=%d",
    status.DBHealthy, status.EmbeddingHealthy, status.VectorCount, status.ItemCount)
if status.EmbeddingError != "" {
    log.Printf("WARNING: Embedding service unavailable — search will be keyword-only: %s",
        status.EmbeddingError)
}
```

#### 2.4 Update mock Embedder in tests

Add `Ping()` to `MockEmbedder` in `engine_test.go`:

```go
func (m *MockEmbedder) Ping(ctx context.Context) error {
    if m.FailOnPing {
        return fmt.Errorf("mock embedding unavailable")
    }
    return nil
}
```

### Testing

- **Unit test (local_test.go):** `TestPing_Success` — httptest server returns
  200, verify `Ping()` returns nil.
- **Unit test (local_test.go):** `TestPing_Unreachable` — httptest server that
  closes immediately, verify `Ping()` returns error within 5s.
- **Unit test (engine_test.go):** `TestHealthCheck_AllHealthy` — all mocks pass,
  verify all fields true.
- **Unit test (engine_test.go):** `TestHealthCheck_EmbeddingDown` — mock Ping
  fails, verify `EmbeddingHealthy == false` and `EmbeddingError` is set, but
  `DBHealthy == true`.

### Risks

- **Adding `Ping()` to the `Embedder` interface is a breaking change.** All
  implementations and mocks must be updated. This is a one-time cost contained
  within the codebase (no external consumers). The stub embedder in
  `server_test.go` and the mock in `engine_test.go` both need the method.
- **The "ping" embed request costs one Ollama inference.** This is negligible
  (nomic-embed-text processes a 4-character string in <50ms) and happens once at
  startup.

---

## Change 3: Structured Logging with `log/slog`

**Tier:** Should-have
**Files:** `codex/internal/mcp/server.go`, `codex/internal/mcp/tools.go`, `codex/internal/core/engine.go`, `codex/cmd/recall-mcp/main.go`
**Estimated effort:** ~4 hours

### Current Behavior

- `engine.go` uses `log.Printf` for warnings (lines 65, 142, 196)
- `tools.go` has no logging at all
- `server.go` has no logging at all
- `main.go` uses `log.Printf` and `log.Fatalf` for startup

There is no structured data in any log output. There is no request-level
tracing. A developer debugging "search was slow 20 minutes ago" has no
information.

### Proposed Behavior

Add `log/slog` (standard library since Go 1.21) with JSON output to stderr.
Structured fields include: request method, tool name, query, duration, result
count, error, session ID.

### Specification

#### 3.1 Initialize slog in main.go

```go
func main() {
    // Structured JSON logging to stderr (stdout is reserved for MCP protocol)
    logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    slog.Info("recall-mcp starting", "version", Version)
    // ... rest of main
}
```

**Critical:** MCP protocol uses stdout. All logging MUST go to stderr.

#### 3.2 Add request logging to MCP server

In `server.go`, add timing and structured logging to `handleCallTool()`:

```go
func (s *Server) handleCallTool(ctx context.Context, req *Request) *Response {
    start := time.Now()

    var params CallToolParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        slog.Warn("invalid tool params", "request_id", req.ID, "error", err)
        return &Response{/* ... existing error ... */}
    }

    handler := NewToolHandler(s.engine, s.sessionID)
    result, err := handler.Handle(ctx, params.Name, params.Arguments)

    duration := time.Since(start)
    if err != nil {
        slog.Error("tool call failed",
            "tool", params.Name,
            "session_id", s.sessionID,
            "duration_ms", duration.Milliseconds(),
            "error", err,
        )
        return &Response{/* ... existing error response ... */}
    }

    slog.Info("tool call",
        "tool", params.Name,
        "session_id", s.sessionID,
        "duration_ms", duration.Milliseconds(),
    )
    // ... existing response building
}
```

#### 3.3 Add search-specific logging to tools.go

In `handleSearch()`, after the search completes:

```go
slog.Info("recall_search",
    "session_id", h.sessionID,
    "query_len", len(query),
    "types", types,
    "scope", scope,
    "result_count", len(results),
    "top_score", topScore,
    "degraded", anyDegraded(results),
)
```

#### 3.4 Replace `log.Printf` in engine.go

Replace the three existing `log.Printf` calls with structured equivalents:

```go
// Line 65 (reranker warning)
slog.Warn("reranker not available", "error", rErr)

// Line 142 (keyword search warning)
slog.Warn("keyword search failed, using vector-only results", "error", err)

// Line 196 (reranking failed)
slog.Warn("reranking failed, using pre-rerank scores", "error", err)
```

#### 3.5 Log format

All log lines are JSON to stderr:

```json
{"time":"2026-03-01T10:15:03Z","level":"INFO","msg":"tool call","tool":"recall_search","session_id":"abc123","duration_ms":245}
{"time":"2026-03-01T10:15:03Z","level":"INFO","msg":"recall_search","session_id":"abc123","query_len":42,"result_count":7,"top_score":0.89,"degraded":false}
{"time":"2026-03-01T10:15:08Z","level":"WARN","msg":"keyword search failed, using vector-only results","error":"FTS5 syntax error"}
```

### What NOT to log

- Query text (may contain sensitive project details)
- Result content (same reason)
- Embedding vectors (binary noise)
- Full request/response JSON (too verbose for default level)

Query text IS logged to the flight recorder (tools.go:97) which is
inside the codex DB and controlled by the project owner. The slog output goes to
stderr which may be captured by process supervisors.

### Testing

- **Integration test:** Capture stderr output during a `server_test.go` test run.
  Verify JSON structure is parseable. Verify duration_ms is present and > 0.
- **No new test files needed.** Logging is verified by inspection and by running
  existing tests with `2>stderr.log`.

### Risks

- **Replacing `log.Printf` with `slog` in engine.go changes import
  dependencies.** `log/slog` is stdlib since Go 1.21, and the project targets
  Go 1.22+, so no compatibility issue.
- **JSON logging to stderr may interfere with MCP parent process.** Claude Code
  launches MCP servers via stdio and captures stderr for error reporting. JSON
  lines on stderr should be safe — Claude Code already handles MCP servers that
  log to stderr.

---

## Change 4: Embedding Timeout Reduction and Circuit Breaker

**Tier:** Should-have
**Files:** `codex/internal/embedding/local.go`
**Estimated effort:** ~3 hours

### Current Behavior

- HTTP client timeout: 30 seconds (local.go:51)
- Max retries: 5 (local.go:17)
- Backoff: 1s, 2s, 4s, 8s, 16s (31 seconds total)
- Worst case for a struggling Ollama: 30s timeout × 5 retries + 31s backoff = **181 seconds**

### Proposed Behavior

- HTTP client timeout: **10 seconds** (sufficient for nomic-embed-text which
  typically responds in <500ms)
- Max retries: **3** (down from 5)
- Backoff: 500ms, 1s, 2s (3.5 seconds total)
- Circuit breaker: after 3 consecutive failures, fast-fail for 30 seconds
- **Worst case: 10s × 3 retries + 3.5s backoff = 33.5 seconds**, then circuit
  opens and subsequent calls fail immediately

### Specification

#### 4.1 Reduce timeout and retry constants

```go
const (
    defaultLocalBaseURL = "http://localhost:11434/api/embed"
    defaultLocalModel   = "nomic-embed-text"
    localMaxRetries     = 3
    localInitialDelay   = 500 * time.Millisecond
    localHTTPTimeout    = 10 * time.Second
)
```

Update `NewLocalClient()`:

```go
client: &http.Client{Timeout: localHTTPTimeout},
```

#### 4.2 Add circuit breaker to LocalClient

```go
type LocalClient struct {
    baseURL string
    model   string
    client  *http.Client

    // Circuit breaker state
    mu              sync.Mutex
    consecutiveFail int
    lastFailTime    time.Time
}

const (
    circuitBreakerThreshold = 3           // consecutive failures to open circuit
    circuitBreakerCooldown  = 30 * time.Second // how long circuit stays open
)

// circuitOpen returns true if the circuit breaker is open (fast-fail mode).
func (c *LocalClient) circuitOpen() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.consecutiveFail < circuitBreakerThreshold {
        return false
    }
    if time.Since(c.lastFailTime) > circuitBreakerCooldown {
        // Cooldown expired — allow a probe request
        c.consecutiveFail = 0
        return false
    }
    return true
}

// recordSuccess resets the circuit breaker.
func (c *LocalClient) recordSuccess() {
    c.mu.Lock()
    c.consecutiveFail = 0
    c.mu.Unlock()
}

// recordFailure increments the consecutive failure counter.
func (c *LocalClient) recordFailure() {
    c.mu.Lock()
    c.consecutiveFail++
    c.lastFailTime = time.Now()
    c.mu.Unlock()
}
```

#### 4.3 Integrate circuit breaker into embed()

```go
func (c *LocalClient) embed(ctx context.Context, text string) ([]float32, error) {
    if c.circuitOpen() {
        return nil, fmt.Errorf("embedding circuit breaker open: service unavailable (cooldown %v)", circuitBreakerCooldown)
    }

    // ... existing request building ...

    var lastErr error
    for attempt := 0; attempt < localMaxRetries; attempt++ {
        // ... existing retry loop ...
    }

    c.recordFailure()
    return nil, fmt.Errorf("max retries (%d) exceeded: %w", localMaxRetries, lastErr)
}
```

On success (line 140, before `return embedResp.Embeddings[0], nil`):

```go
c.recordSuccess()
return embedResp.Embeddings[0], nil
```

#### 4.4 Circuit breaker interaction with Ping()

`Ping()` bypasses the circuit breaker — it's explicitly asking "is the service
back?" The health check in Change 2 uses `Ping()`, and we don't want the
circuit breaker to prevent startup health checks.

```go
func (c *LocalClient) Ping(ctx context.Context) error {
    // Ping bypasses circuit breaker — it's a probe to check if service is back
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    vec, err := c.embedDirect(ctx, "ping") // new method: embed without circuit breaker check
    if err != nil {
        return fmt.Errorf("embedding service unreachable at %s: %w", c.baseURL, err)
    }
    _ = vec
    c.recordSuccess() // Successful ping resets the circuit breaker
    return nil
}
```

Factor the HTTP logic into `embedDirect()` (the raw HTTP call) and have
`embed()` wrap it with circuit breaker logic.

### Testing

- **Unit test:** `TestCircuitBreaker_Opens` — fail 3 requests, verify 4th
  returns immediately with "circuit breaker open" error.
- **Unit test:** `TestCircuitBreaker_Resets` — fail 3, wait cooldown, verify
  next request is attempted (not fast-failed).
- **Unit test:** `TestCircuitBreaker_SuccessResets` — fail 2, succeed 1, fail 2
  more, verify circuit doesn't open (counter reset on success).
- **Unit test:** `TestPing_BypassesCircuitBreaker` — circuit is open, Ping
  succeeds, verify embed() works again.
- **Existing tests:** `TestRetryOn5xx`, `TestNoRetryOn4xx`,
  `TestMaxRetriesExhausted`, `TestContextCancellation` — verify these still pass
  with reduced constants.

### Risks

- **30-second cooldown may be too aggressive.** If Ollama restarts quickly (e.g.,
  Docker restart policy), users may experience 30 seconds of keyword-only search
  unnecessarily. Mitigated by: (a) Ping bypasses the breaker, so a manual health
  check or new session resets it, and (b) 30 seconds is short enough to be
  tolerable.
- **Reducing HTTP timeout from 30s to 10s may cause failures on first-request
  model loading.** Ollama lazy-loads models; the first request after a cold start
  can take 15-30 seconds as the model loads into GPU memory. Mitigate by: making
  the Ping() timeout longer (10s) or documenting that the first session after
  Ollama restart may see a brief keyword-only period.

---

## Change 5: Vector Cache Invalidation

**Tier:** Should-have
**Files:** `codex/internal/storage/vecstore.go`
**Estimated effort:** ~2 hours

### Current Behavior

`VecStore` loads all vectors into memory at startup via `loadAll()`
(vecstore.go:41). The in-memory map is updated when the same process calls
`Upsert()` or `Delete()` (lines 86-105, 156-167). But vectors added by
**other processes** (e.g., a second concurrent session) are invisible until
restart.

This manifests as: "I added knowledge in session A, but session B's search
doesn't find it."

### Proposed Behavior

Periodically reload vectors from SQLite to pick up external changes. Reload
on every Search() call if the vector count in SQLite has changed since last
load.

### Specification

#### 5.1 Add change detection to VecStore

```go
type VecStore struct {
    db *sql.DB

    mu      sync.RWMutex
    vectors map[string][]float32

    // Change detection
    lastKnownCount int
    lastCheckTime  time.Time
}

const vectorCacheCheckInterval = 10 * time.Second
```

#### 5.2 Add stale check method

```go
// checkStale queries the vectors table count and reloads if it differs from
// the in-memory count. Rate-limited to avoid hammering SQLite on every search.
func (vs *VecStore) checkStale() {
    if time.Since(vs.lastCheckTime) < vectorCacheCheckInterval {
        return
    }

    var count int
    err := vs.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&count)
    if err != nil {
        return // Silently skip — the cache is still usable
    }
    vs.lastCheckTime = time.Now()

    vs.mu.RLock()
    currentCount := len(vs.vectors)
    vs.mu.RUnlock()

    if count != currentCount {
        // External change detected — reload
        newVecs := make(map[string][]float32)
        rows, err := vs.db.Query("SELECT item_id, embedding, dimensions FROM vectors")
        if err != nil {
            return
        }
        defer rows.Close()

        for rows.Next() {
            var id string
            var blob []byte
            var dims int
            if err := rows.Scan(&id, &blob, &dims); err != nil {
                return
            }
            vec, err := blobToFloat32(blob, dims)
            if err != nil {
                return
            }
            newVecs[id] = vec
        }
        if rows.Err() != nil {
            return
        }

        vs.mu.Lock()
        vs.vectors = newVecs
        vs.lastKnownCount = count
        vs.mu.Unlock()

        slog.Info("vector cache reloaded", "count", count, "reason", "external_change")
    }
}
```

#### 5.3 Call checkStale before Search

```go
func (vs *VecStore) Search(ctx context.Context, queryVec []float32, limit int) ([]ScoredResult, error) {
    vs.checkStale()

    // ... existing implementation unchanged
}
```

### Design rationale

**Why count-based detection instead of unconditional reload?**

`loadAll()` reads every vector from SQLite and deserializes them. For a 1000-
document corpus with 768-dim vectors, that's ~3MB of BLOB I/O. At <10K
documents this takes <50ms, but we don't want to do it on every search. The
`SELECT COUNT(*)` query costs <1ms and tells us whether anything changed.

**Why 10-second check interval?**

A session adds knowledge via `/end`, which is an infrequent event (once per
session, maybe every 30-60 minutes). A 10-second staleness window is
imperceptible. The interval is conservative enough to add zero measurable
overhead to the search hot path.

**Why silent failure?**

If the reload query fails, the existing in-memory vectors are still valid for
this process's own data. Failing loudly would degrade the search experience
for a cache consistency issue that is, at worst, a missing recent item.

### Testing

- **Unit test:** `TestVecStore_DetectsExternalInsert` — create two `VecStore`
  instances on the same DB. Insert via instance A. Call `Search()` on instance B.
  Verify B finds the new vector within one check cycle.
- **Unit test:** `TestVecStore_DetectsExternalDelete` — same setup, delete via A,
  verify B's search no longer returns the item.
- **Unit test:** `TestVecStore_RateLimitsCheck` — call Search twice within 10
  seconds, verify only one `SELECT COUNT(*)` was executed (via query counting or
  mock).

### Risks

- **Race condition on reload.** The `mu.Lock()` in the reload path blocks
  ongoing searches. At <10K vectors, the reload takes <50ms, which is acceptable.
  For very large corpora this could cause latency spikes, but the <10K scale
  target makes this moot.
- **Count-based detection misses replace operations.** If process A deletes one
  vector and adds a different one, the count is unchanged. The cache stays stale.
  This is an unlikely edge case for knowledge management workloads — items are
  added far more often than replaced across sessions.

---

## Change 6: Tighten Eval Thresholds

**Tier:** Improve
**Files:** `codex/eval/harness_test.go`
**Estimated effort:** ~30 minutes

### Current Thresholds (harness_test.go:79-84)

| Metric | Threshold | Actual Performance |
|--------|-----------|--------------------|
| Recall@10 | >= 0.3 | 0.908 |
| MRR | >= 0.2 | 0.842 |

A regression to 0.31 Recall@10 (a 66% degradation from current performance)
would still pass.

### Proposed Thresholds

| Metric | New Threshold | Rationale |
|--------|---------------|-----------|
| Recall@10 | >= 0.70 | ~23% below current; catches significant regressions |
| Recall@5 | >= 0.50 | New check; top-5 results should contain half of relevant docs |
| Precision@5 | >= 0.30 | New check; at least 30% of top-5 should be relevant |
| nDCG@10 | >= 0.50 | New check; ranking quality floor |
| MRR | >= 0.55 | ~35% below current; first relevant result should be near top |

### Specification

Replace the assertion block in `harness_test.go`:

```go
// Regression thresholds — set ~25% below current benchmark to catch
// significant quality drops without being flaky on minor variations.
if summary.RecallAt10 < 0.70 {
    t.Errorf("Recall@10 regression: %.3f (min 0.70, benchmark 0.908)", summary.RecallAt10)
}
if summary.RecallAt5 < 0.50 {
    t.Errorf("Recall@5 regression: %.3f (min 0.50)", summary.RecallAt5)
}
if summary.PrecisionAt5 < 0.30 {
    t.Errorf("Precision@5 regression: %.3f (min 0.30)", summary.PrecisionAt5)
}
if summary.NDCGAt10 < 0.50 {
    t.Errorf("nDCG@10 regression: %.3f (min 0.50)", summary.NDCGAt10)
}
if summary.MRRScore < 0.55 {
    t.Errorf("MRR regression: %.3f (min 0.55, benchmark 0.842)", summary.MRRScore)
}
```

### Why not tighter?

Embedding quality varies slightly across Ollama versions and hardware
(quantization levels, GPU vs CPU). Setting thresholds at 90% of benchmark would
cause flaky tests on different machines. The ~25% margin is enough to catch
meaningful regressions (broken tokenization, wrong embedding model, RRF
parameter changes) without false positives.

### Testing

- Run the eval suite once with current code to verify all new thresholds pass.
- Intentionally break something (e.g., remove query prefix in local.go) and
  verify the tighter thresholds catch it while the old ones wouldn't.

---

## Implementation Order

These changes have the following dependency graph:

```
Change 1 (FTS5 fallback)     ← no dependencies
Change 2 (Health check)      ← depends on Change 1 (fallback makes health check non-blocking)
Change 3 (Structured logging) ← no dependencies
Change 4 (Circuit breaker)   ← no dependencies, but should follow Change 1
Change 5 (Cache invalidation) ← no dependencies
Change 6 (Eval thresholds)   ← no dependencies
```

Recommended order:

1. **Change 1** — FTS5 fallback (unblocks Change 2)
2. **Change 4** — Circuit breaker (builds on same embedding code)
3. **Change 2** — Health check (uses Ping from Change 4, degraded mode from Change 1)
4. **Change 3** — Structured logging (touch the same files as 1/2, easier to do after)
5. **Change 5** — Vector cache invalidation (independent)
6. **Change 6** — Eval thresholds (independent, quick)

Total estimated effort: **~13 hours** of focused work (2 days).

## What This Does NOT Address

These changes are specifically scoped to production quality for a small team.
The following are explicitly out of scope:

- **Multi-tenancy / RBAC** — Not needed for a trusted team
- **Horizontal scaling** — SQLite WAL handles 3-5 concurrent readers fine
- **PostgreSQL migration** — Only needed if the team grows past ~10K documents
- **Hosted embedding providers** — Ollama is fine for local use; provider
  abstraction is a separate decision
- **Container packaging** — `make install` is fine for a small team
- **Reranking implementation** — The stub is harmless; implementing it is a
  feature, not a hardening concern
