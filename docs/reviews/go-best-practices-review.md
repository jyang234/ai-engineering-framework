# Go Best Practices Review: AI Engineering Framework

**Date**: 2026-02-26
**Scope**: All Go code in `edi/` (67 files, ~12K LOC) and `codex/` (69 files, ~22K LOC)
**Reference**: Effective Go, Go Code Review Comments, Go Proverbs

---

## Executive Summary

The codebase is **generally well-written Go** with clean architecture, proper use of interfaces, and good error handling practices. The project follows standard Go layout conventions and demonstrates clear understanding of idiomatic patterns. However, there are several areas where adherence to Effective Go and community best practices could be improved.

**Overall Grade: B+**

| Category | Grade | Notes |
|----------|-------|-------|
| Project Layout | A | Standard `cmd/`, `internal/`, `pkg/` structure |
| Naming Conventions | A- | Mostly idiomatic; minor inconsistencies |
| Error Handling | B+ | Good wrapping; missing sentinel errors |
| Interface Design | B+ | Clean interfaces; `MetadataStorage` too fat; MCP uses concrete types |
| Concurrency | B | Proper mutexes; MCP context issue; slice corruption bug |
| Testing | B+ | Good core tests; zero MCP test coverage |
| Resource Management | B+ | Proper defer usage; temp dir leak in eval |
| Code Organization | B | Significant type duplication across modules |
| Documentation | B- | Many exported types lack doc comments |
| Consistency | B- | Mixed `interface{}`/`any`, mixed logging |

---

## Strengths

### 1. Excellent Interface Design (`codex/internal/core/interfaces.go`)

The `core` package defines clean, focused interfaces following the "accept interfaces, return structs" principle:

```go
type Embedder interface {
    EmbedDocument(ctx context.Context, text string) ([]float32, error)
    EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

type VectorStorage interface {
    Upsert(ctx context.Context, itemID string, vector []float32) error
    Search(ctx context.Context, queryVec []float32, limit int) ([]storage.ScoredResult, error)
    Delete(ctx context.Context, itemID string) error
}
```

These are small, focused interfaces (2-3 methods each), making them easy to implement and mock. The `SearchEngine` accepts these interfaces, enabling full testability via `NewSearchEngineWithDeps()`.

### 2. Dependency Injection & Testability

The constructor pattern with a deps struct is excellent:

```go
// codex/internal/core/engine.go:82
func NewSearchEngineWithDeps(deps SearchEngineDeps) *SearchEngine { ... }
```

The `embedding.LocalClient` uses the functional options pattern cleanly:

```go
// codex/internal/embedding/local.go:47
func NewLocalClient(opts ...LocalClientOption) *LocalClient { ... }
```

### 3. Consistent Error Wrapping

Throughout the codebase, errors are properly wrapped with `%w` and contextual messages:

```go
return nil, fmt.Errorf("failed to open metadata store: %w", err)
return nil, fmt.Errorf("vecstore migrate: %w", err)
return nil, fmt.Errorf("failed to embed query: %w", err)
```

### 4. Proper Resource Cleanup

`defer` is used correctly and consistently for database rows, files, and trees:

```go
defer rows.Close()
defer tree.Close()
defer indexer.Close()
defer tx.Rollback()  // in edi/internal/recall/storage.go:276
```

### 5. Well-Structured Tests

Tests follow BDD-style naming and Given/When/Then structure:

```go
t.Run("Given items exist When List called with type filter Then returns only matching type", func(t *testing.T) {
    // Given
    metaStore := NewMockMetadataStorage()
    // ...
    // When
    items, err := engine.List(ctx, "pattern", "", 10, 0)
    // Then
    if err != nil { t.Fatalf("List failed: %v", err) }
})
```

Mock implementations are thorough with configurable failure modes (`FailOnSave`, `FailOnUpsert`, etc.).

### 6. Proper Concurrency in VecStore

`codex/internal/storage/vecstore.go` correctly uses `sync.RWMutex` with `RLock` for reads and `Lock` for writes. The min-heap based top-K search is algorithmically sound.

---

## Issues Found

### Critical

#### C1. Slice Corruption Bug in `eval/condition.go`

**File**: `codex/eval/condition.go:124`

```go
var baseAllowedTools = []string{
    "Edit", "Write", "Read", "Glob", "Grep",
    "Bash(go build:*)", "Bash(go test:*)", "Bash(go vet:*)",
    "Bash(gofumpt:*)", "Bash(golangci-lint:*)", "Bash(go mod tidy:*)",
}

// In newAEFFull():
tools := append(baseAllowedTools, recallTools...)
```

This is a **real bug**. Since `baseAllowedTools` is a package-level `var`, if the Go runtime allocated the backing array with capacity > length, `append` will mutate the underlying array, corrupting `baseAllowedTools` for all subsequent callers. This would cause `newBaseline()` and `newAEFMinimal()` to silently include recall tools.

**Fix**:
```go
tools := make([]string, len(baseAllowedTools), len(baseAllowedTools)+len(recallTools))
copy(tools, baseAllowedTools)
tools = append(tools, recallTools...)
```

Or make `baseAllowedTools` a function returning a fresh slice each time.

#### C2. MCP Server Context Cancellation Is Ineffective

**Files**: `edi/internal/recall/server.go:98-131`, `codex/internal/mcp/server.go:100-133`

Both MCP server `Run` methods have a broken context cancellation pattern:

```go
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    line, err := reader.ReadBytes('\n')  // BLOCKS indefinitely
    // ...
}
```

The `select` with `default` only checks context at the top of the loop. `ReadBytes` is a blocking call that does not respect context cancellation. If the context is cancelled while blocked on `ReadBytes`, the server will not exit until new input arrives.

**Recommendation**: Run the reader in a separate goroutine and use a channel to multiplex reads with context cancellation, or use `io.ReadCloser` and close the reader when context is done.

### High Priority

#### H1. Significant Type Duplication Between Modules

**Files**: `edi/internal/recall/server.go` vs `codex/internal/mcp/server.go`

The following types are fully duplicated across both modules:
- `MCPRequest`, `MCPResponse`, `MCPError`
- `InitializeResult`, `ServerInfo`, `ServerCapabilities`, `ToolsCapability`
- `ListToolsResult`, `Tool`, `CallToolParams`, `CallToolResult`, `ToolContent`
- `generateID()` function

**Impact**: Any protocol change must be updated in two places. This violates DRY and increases maintenance burden.

**Recommendation**: Extract shared MCP protocol types into a shared package (e.g., `codex/pkg/mcp/types.go` or a new top-level `pkg/mcp/` module), or define them in only one module and import.

#### H2. Missing Sentinel Errors

**Files**: `codex/internal/storage/metadata.go:274`, `codex/internal/core/engine.go:347`

"Not found" errors are created as formatted strings, making them impossible to check programmatically:

```go
// metadata.go:274
return nil, fmt.Errorf("item not found: %s", id)

// engine.go:347
return fmt.Errorf("item not found: %w", err)
```

**Recommendation**: Define sentinel errors per Effective Go:

```go
var ErrNotFound = errors.New("item not found")

// Usage:
return nil, fmt.Errorf("%w: %s", ErrNotFound, id)

// Callers can check:
if errors.Is(err, storage.ErrNotFound) { ... }
```

#### H3. Inconsistent Not-Found Return Conventions

**Files**: `codex/internal/storage/metadata.go`

Two methods in the same package handle "not found" differently:
- `GetItem()` (line 274): returns `(nil, error)` — error for not found
- `FindByTitle()` (line 419): returns `(nil, nil)` — nil for not found

This is confusing for callers. Per Go conventions, pick one pattern and be consistent. The `FindByTitle` approach of `(nil, nil)` is common for "optional lookup" semantics, while `GetItem` should return a sentinel error.

#### H4. Zero Test Coverage for MCP Server Package

**Files**: `codex/internal/mcp/` — no `*_test.go` files exist

The MCP server (`server.go`) and tool handler (`tools.go`) are the primary interface between Claude Code and the Codex backend. There are zero unit tests for this package. Also missing tests: `codex/internal/embedding/`, `codex/internal/reranking/`, `codex/internal/chunking/`, `codex/pkg/recall/`.

**Recommendation**: Add unit tests for `ToolHandler.Handle()` dispatching, each tool handler method, JSON-RPC message parsing, and error responses. The `codex/internal/mcp/server.go:RunForIO` method already accepts `io.Reader`/`io.Writer` for testability — use `io.Pipe` to test the full request/response cycle.

#### H5. MCP Types Stutter Package Name

**Files**: `codex/internal/mcp/server.go:30-46`, `edi/internal/recall/server.go:30-46`

Per Effective Go, types should not repeat the package name. These types all stutter:
- `mcp.MCPRequest` should be `mcp.Request`
- `mcp.MCPResponse` should be `mcp.Response`
- `mcp.MCPError` should be `mcp.Error`

#### H6. MCP ToolHandler/Server Depend on Concrete Types

**File**: `codex/internal/mcp/tools.go:16`, `codex/internal/mcp/server.go:16`

```go
type ToolHandler struct {
    engine    *core.SearchEngine  // concrete type, not interface
    sessionID string
}
```

This makes it impossible to unit test `ToolHandler` with a mock engine. Should accept an interface covering the methods used (`Search`, `Get`, `Add`, `RecordFeedback`, `LogFlightRecorder`).

#### H7. `MetadataStorage` Interface Is Too Fat

**File**: `codex/internal/core/interfaces.go:42-53`

`MetadataStorage` has 9 methods spanning three concerns: item CRUD, feedback, and flight recorder. Per the Interface Segregation Principle, split into:

```go
type ItemStorage interface { SaveItem, GetItem, FindByTitle, ListItems, DeleteItem, CountItemsByType, Close }
type FeedbackRecorder interface { RecordFeedback }
type FlightRecorder interface { LogFlightRecorder, GetFlightRecorderEntries }
```

#### H8. Swallowed Errors in Config Helpers

**File**: `edi/internal/config/loader.go:62-83`

Four exported helper functions silently discard errors:

```go
func GlobalConfigPath() string {
    home, _ := os.UserHomeDir()  // error silently ignored
    return filepath.Join(home, ".edi", "config.yaml")
}
```

If `os.UserHomeDir()` fails, `home` is `""`, producing path `"/.edi/config.yaml"` — a valid but wrong path that could lead to confusing behavior.

**Recommendation**: Either return `(string, error)` or panic on truly unrecoverable errors. At minimum, document the assumption.

#### H5. Variable Shadowing: `ctx` Shadows Package Name

**Files**: `edi/internal/recall/server.go:462`, `codex/internal/mcp/tools.go:233`

```go
func (h *ToolHandler) handleFeedback(args map[string]interface{}) (interface{}, error) {
    // ...
    ctx, _ := args["context"].(string)  // shadows 'context' package
```

The variable `ctx` is idiomatically reserved for `context.Context` in Go. Using it for a string value is confusing.

**Recommendation**: Rename to `feedbackCtx`, `ctxStr`, or `contextStr`.

### Medium Priority

#### M1. Temp Directory Leak in Eval Runner

**File**: `codex/eval/runner_agent.go:158-186`

The `bootMCP` method creates a temp directory at line 159 but only cleans it up in error paths (lines 174, 181). On the success path, there is no `defer os.RemoveAll(tmpDir)`, causing leaked temp directories for every successful eval run.

#### M2. Duplicated Retry Logic and Code Fence Stripping

**Files**: `codex/eval/judge.go:93-144` vs `judge.go:163-206`, `judge.go:212-223` vs `scorer.go:394-403`

The retry-with-exponential-backoff pattern is nearly identical in `RawJudge` and `RawHTTPPost`. The markdown code fence stripping logic is duplicated in `parseJudgment` and `judgeCodeQuality`. Both should be extracted to shared helpers.

#### M3. Hand-Rolled String Functions in Eval

**File**: `codex/eval/runner_agent.go:507-541`

`containsIgnoreCase` and `bytesContains` are hand-rolled ASCII-only implementations. Use `strings.Contains(strings.ToLower(s), strings.ToLower(substr))` for proper Unicode support.

**File**: `codex/eval/runner_pipe.go:306-319` — `splitLines` reimplements `strings.Split(s, "\n")`.

#### M4. `interface{}` vs `any` Inconsistency

The project requires Go 1.22+ but inconsistently uses both `interface{}` and `any`:

- Uses `any`: `codex/internal/core/types.go:54`, `codex/internal/storage/metadata.go:35`
- Uses `interface{}`: `codex/internal/mcp/server.go:34`, `edi/internal/recall/server.go:32`, most of `codex/internal/mcp/tools.go`

**Recommendation**: Use `any` consistently (it's the Go 1.18+ alias for `interface{}`). A simple find-and-replace would unify this.

#### M5. `fmt.Errorf` Without Format Arguments Should Be `errors.New`

Multiple locations create errors with `fmt.Errorf` when no formatting is needed:

```go
// codex/internal/mcp/tools.go:54
return nil, fmt.Errorf("query is required")

// codex/internal/mcp/tools.go:157
return nil, fmt.Errorf("type, title, and content are required")
```

**Recommendation**: Use `errors.New("query is required")` when there are no format arguments. This is slightly more efficient and signals intent more clearly.

#### M6. Magic Numbers Throughout

**File**: `codex/internal/core/engine.go`

```go
candidateLimit := 50                    // line 116 - why 50?
if candidateLimit < 20 {                // line 119 - why 20?
    candidateLimit = 20
}
results := reciprocalRankFusion(..., 60)  // line 161 - RRF k=60

// codex/internal/mcp/server.go:101
reader := bufio.NewReaderSize(r, 10<<20)  // 10MB buffer - why?

// codex/eval/scorer.go:367
if len(implementation) > 50000 {          // 50K chars - why?
```

**Recommendation**: Extract to named constants with documentation:

```go
const (
    defaultCandidateLimit = 50  // Over-fetch for RRF fusion
    minCandidateLimit     = 20  // Minimum for reasonable fusion quality
    rrfK                  = 60  // Standard RRF smoothing constant
)
```

#### M7. Large Functions Could Be Decomposed

- `codex/internal/core/engine.go:111-225` — `Search()` is 114 lines with 9 numbered steps
- `edi/internal/cli/launch.go:17-123` — `runLaunch()` is 106 lines of sequential operations
- `codex/eval/scorer.go:264-307` — `checkPitfalls()` has complex nested conditionals

While the numbered comments in `Search()` help readability, these functions would benefit from being broken into smaller, named helpers that each do one thing.

#### M8. Inconsistent Logging Strategy

Three different logging approaches are used:

1. `log.Printf` (standard library) — majority of codebase
2. `log/slog` (structured logging) — `codex/internal/web/handlers.go`
3. `fmt.Fprintf(os.Stderr, ...)` — `edi/internal/cli/` package

**Recommendation**: Standardize on `log/slog` (available since Go 1.21, within the 1.22+ requirement). It provides structured logging with levels, which is more appropriate for a production tool.

#### M9. Compensating Actions Without Transactions

**File**: `codex/internal/core/engine.go:237-256`

The `Add` method performs a manual compensating delete if vector storage fails:

```go
if err := e.metadata.SaveItem(itemToRecord(item)); err != nil {
    return fmt.Errorf("failed to save metadata: %w", err)
}
if err := e.vecStore.Upsert(ctx, item.ID, vec); err != nil {
    _ = e.metadata.DeleteItem(item.ID)  // compensating action, error ignored
    return fmt.Errorf("failed to store vector: %w", err)
}
```

This is fragile: if `DeleteItem` fails, the database is left in an inconsistent state (metadata exists without a vector). The same pattern appears in `Update()`.

**Recommendation**: Since both stores share the same SQLite database, wrap operations in a SQL transaction. Alternatively, document the acceptable inconsistency window.

#### M10. Repeated `os.Getwd()` Calls in Same Flow

During a single launch, `os.Getwd()` is called independently in:
- `edi/internal/cli/launch.go:25`
- `edi/internal/briefing/generator.go:43`
- `edi/internal/config/loader.go:22`

**Recommendation**: Resolve `cwd` once at the top of the launch flow and pass it as a parameter.

### Low Priority

#### L1. Missing Doc Comments on Exported MCP Types

**Files**: `edi/internal/recall/server.go:30-95`, `codex/internal/mcp/server.go:30-91`

Per Effective Go: "Every exported name in a program should have a doc comment." The MCP protocol types (`MCPRequest`, `MCPResponse`, `MCPError`, `Tool`, `CallToolParams`, etc.) lack doc comments.

While these are in `internal/` packages, doc comments aid maintainability.

#### L2. `Metadata` Type Alias Is Ambiguous

**File**: `edi/internal/tasks/manifest.go:33`

```go
type Metadata map[string]interface{}
```

This creates a named type for `map[string]interface{}` but doesn't add methods or validation. The name `Metadata` is generic and could conflict with other packages. Consider just using `map[string]any` inline, or add meaningful methods to justify the type.

#### L3. `ASTChunker.available` Field Is Always True

**File**: `codex/internal/chunking/ast.go:22,27`

```go
type ASTChunker struct {
    // ...
    available bool
}

func NewASTChunker() *ASTChunker {
    chunker := &ASTChunker{available: true}  // always true
    // ...
}
```

The `available` field is always set to `true` in the constructor and never changes. The `IsAvailable()` method always returns `true`. This appears to be dead code from a feature flag that was never needed.

**Recommendation**: Remove the field and `IsAvailable()` method, or implement actual availability checking (e.g., verify tree-sitter bindings loaded successfully).

#### L4. `DetectLanguage` Uses Full Path Instead of Extension

**File**: `codex/internal/chunking/ast.go:388`

```go
func DetectLanguage(filePath string) string {
    ext := strings.ToLower(filePath)  // lowercases entire path
    switch {
    case strings.HasSuffix(ext, ".go"):
```

This works but is wasteful — it lowercases the entire file path when only the extension matters. Use `filepath.Ext()`:

```go
func DetectLanguage(filePath string) string {
    switch strings.ToLower(filepath.Ext(filePath)) {
    case ".go":
        return "go"
    // ...
    }
}
```

#### L5. No `.golangci.yml` Configuration

The Makefiles reference `golangci-lint` but no configuration file exists. Running with defaults may miss project-specific issues or flag irrelevant ones.

**Recommendation**: Add a `.golangci.yml` with at minimum:
- Enable `errcheck`, `govet`, `staticcheck`, `gosimple`
- Enable `gofumpt` (since it's the project standard)
- Enable `exhaustive` for switch completeness
- Consider `contextcheck` for proper context propagation

#### L6. `ClaudeTask.MarshalJSON` Is Redundant

**File**: `edi/internal/tasks/manifest.go:82-85`

```go
func (ct *ClaudeTask) MarshalJSON() ([]byte, error) {
    type Alias ClaudeTask
    return json.Marshal((*Alias)(ct))
}
```

This custom `MarshalJSON` delegates to the default behavior via an alias — it does nothing that `json.Marshal` wouldn't already do. Remove it unless there's a planned divergence.

#### L7. `collectGoFiles` Reads All Files Into Memory

**File**: `codex/eval/scorer.go:309-324`

```go
func collectGoFiles(dir string) []string {
    var contents []string
    _ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        // ... reads every .go file entirely into memory
    })
    return contents
}
```

This loads all Go source files into memory simultaneously. For large projects being evaluated, this could be significant. Also, the function is called twice during scoring (once in `checkPitfalls`, once in `judgeCodeQuality`).

**Recommendation**: Call once and pass the result, or use `filepath.WalkDir` (more efficient than `filepath.Walk`).

---

## Positive Patterns Worth Preserving

1. **Interface segregation in `core/interfaces.go`** — keep interfaces small and focused
2. **Error wrapping with `%w`** — maintains error chains for debugging
3. **Functional options pattern** in `embedding.LocalClient` — extensible configuration
4. **Exponential backoff retry** in `embedding/local.go` and `eval/judge.go` — robust HTTP clients
5. **Min-heap for top-K search** in `vecstore.go` — algorithmically optimal
6. **BDD-style test naming** — descriptive and self-documenting
7. **`doc.go` files** in packages — good for package-level documentation
8. **Build tag separation** (`fts5`, `evalintegration`) — clean build boundaries

---

## Recommended Priority Actions

1. **Fix slice corruption bug** in `eval/condition.go:124` (Critical — data corruption)
2. **Fix MCP server context cancellation** (Critical — affects graceful shutdown)
3. **Add MCP server unit tests** — zero coverage on the primary Claude Code interface (High)
4. **Define sentinel errors** for not-found and other domain errors (High)
5. **Extract shared MCP types** to eliminate duplication; fix name stuttering (High)
6. **Fix temp directory leak** in `eval/runner_agent.go` (High)
7. **Split `MetadataStorage` interface** — 9 methods spanning 3 concerns (High)
8. **Make MCP ToolHandler accept interfaces** instead of concrete `*SearchEngine` (High)
9. **Add `.golangci.yml`** with project-specific rules (Low effort, high value)
10. **Standardize on `any` over `interface{}`** (Low effort, consistency win)
11. **Extract duplicated retry/parsing helpers** in eval package (Medium)
12. **Standardize on `log/slog`** across the codebase (Medium effort)
13. **Fix swallowed errors** in config path helpers (Medium)
