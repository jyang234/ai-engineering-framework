# Go Best Practices Review: AI Engineering Framework

**Date**: 2026-02-26 (updated)
**Scope**: All Go code in `edi/` (67 files, ~12K LOC) and `codex/` (69 files, ~22K LOC)
**Reference**: Effective Go, Go Code Review Comments, Go Proverbs

---

## Executive Summary

The codebase is **generally well-written Go** with clean architecture, proper use of interfaces, and good error handling practices. The project follows standard Go layout conventions and demonstrates clear understanding of idiomatic patterns. However, a thorough file-by-file analysis uncovered one real bug, several structural issues, and widespread inconsistencies that should be addressed.

**Overall Grade: B**

| Category | Grade | Notes |
|----------|-------|-------|
| Project Layout | A | Standard `cmd/`, `internal/`, `pkg/` structure |
| Naming Conventions | B+ | Mostly idiomatic; MCP stutter; `ctx` shadowing |
| Error Handling | B | Good wrapping; missing `errors.Is`; 20+ swallowed `os.UserHomeDir()` |
| Interface Design | B | Clean core interfaces; MCP/Server use concrete types; `Backend` incomplete |
| Concurrency | B | Proper mutexes; MCP context issue; slice corruption bug |
| Testing | B | Good core/eval tests; zero MCP test coverage; unsafe assertions in tests |
| Resource Management | B+ | Proper defer usage; temp dir leak in eval; inconsistent `Close()` error handling |
| Code Organization | B- | Significant duplication across modules (types, helpers, frontmatter parsing) |
| Documentation | B- | Many exported types lack doc comments; stale TODOs |
| Consistency | C+ | Mixed `interface{}`/`any`, 3 logging strategies, inconsistent error conventions |

---

## Strengths

### 1. Clean Interface Design (`codex/internal/core/interfaces.go`)

The `core` package defines small, focused interfaces following the "accept interfaces, return structs" principle:

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

2-3 methods per interface, easy to implement and mock. `NewSearchEngineWithDeps()` enables full testability.

### 2. Dependency Injection & Testability

```go
// codex/internal/core/engine.go:82
func NewSearchEngineWithDeps(deps SearchEngineDeps) *SearchEngine { ... }
```

Functional options pattern in `embedding.LocalClient` and `web.ServerOption`. Clean constructor patterns throughout.

### 3. Consistent Error Wrapping

Throughout the codebase, errors are properly wrapped with `%w` and contextual messages:

```go
return nil, fmt.Errorf("failed to open metadata store: %w", err)
return nil, fmt.Errorf("vecstore migrate: %w", err)
```

### 4. Proper Resource Cleanup

`defer` used correctly and consistently:

```go
defer rows.Close()              // database queries (storage/metadata.go)
defer tx.Rollback()             // transactions (edi/internal/recall/storage.go:276)
defer tree.Close()              // tree-sitter (chunking/ast.go)
```

Atomic file writes via temp file + rename in `edi/internal/tasks/sync.go:92-101`. Named return + deferred closure for `Close()` error capture in `edi/internal/launch/commands.go:106-125`.

### 5. Well-Structured Tests

BDD-style naming with Given/When/Then structure:

```go
t.Run("Given items exist When List called with type filter Then returns only matching type", ...)
```

Proper use of `t.TempDir()`, `t.Setenv()`, `t.Helper()`, build tags (`fts5`, `evalintegration`), and configurable mock failure modes.

### 6. Excellent Package Documentation

Outstanding `doc.go` files in `tasks`, `briefing`, `recall`, `memory` packages documenting strategy, data flow, file formats, and usage.

### 7. Compile-Time Interface Checks

```go
// edi/internal/recall/backend.go:22
var _ Backend = (*Storage)(nil)
```

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

**Impact**: Any protocol change must be updated in two places.

**Recommendation**: Extract shared MCP protocol types into a shared package (e.g., `codex/pkg/mcp/types.go`).

#### H2. Pervasive `errors.Is` Violations

Direct error comparison instead of `errors.Is` in multiple files:

| File | Line | Error |
|------|------|-------|
| `codex/internal/mcp/server.go` | 114 | `err == io.EOF` |
| `codex/internal/storage/metadata.go` | 273, 418 | `err == sql.ErrNoRows` |
| `codex/cmd/recall-mcp/main.go` | 53 | `err != context.Canceled` |
| `edi/internal/recall/server.go` | 112 | `err == io.EOF` |
| `edi/internal/recall/storage.go` | 185 | `err == sql.ErrNoRows` |

**Fix**: Use `errors.Is(err, target)` in all cases. Direct comparison breaks when errors are wrapped.

#### H3. Missing Sentinel Errors & Inconsistent Not-Found Convention

**Files**: `codex/internal/storage/metadata.go`

Two methods handle "not found" differently:
- `GetItem()` (line 274): returns `(nil, error)` — error for not found
- `FindByTitle()` (line 419): returns `(nil, nil)` — nil for not found

No sentinel error exists, making programmatic error checking impossible:

```go
return nil, fmt.Errorf("item not found: %s", id)  // can't check with errors.Is
```

**Recommendation**: Define `var ErrNotFound = errors.New("item not found")` and use consistently.

#### H4. Zero Test Coverage for MCP Server Packages

**Files**: `codex/internal/mcp/` — no `*_test.go` files exist

The MCP server and tool handler are the primary interface between Claude Code and the Codex backend with zero unit tests. Also missing direct tests: `codex/internal/embedding/`, `codex/internal/reranking/`, `codex/internal/chunking/`, `codex/pkg/recall/`.

**Recommendation**: Add unit tests using `io.Pipe` against `RunForIO`. Requires interface fix (H6) for proper mock injection.

#### H5. MCP Types Stutter Package Name

**Files**: `codex/internal/mcp/server.go:30-46`, `edi/internal/recall/server.go:30-46`

Per Effective Go, types should not repeat the package name:
- `mcp.MCPRequest` → `mcp.Request`
- `mcp.MCPResponse` → `mcp.Response`
- `mcp.MCPError` → `mcp.Error` (or `mcp.RPCError` to avoid collision with builtin)

#### H6. MCP Server/ToolHandler Depend on Concrete Types

**Files**: `codex/internal/mcp/tools.go:16`, `codex/internal/mcp/server.go:16`, `codex/internal/web/server.go:21`

```go
type ToolHandler struct {
    engine    *core.SearchEngine  // concrete type, not interface
    sessionID string
}
```

Also: `edi/internal/recall/server.go:16-19` accepts concrete `*Storage` instead of `Backend` interface.

Makes unit testing impossible without a full SearchEngine/Storage with SQLite. The `web` package even defines `SearchEngineInterface` in its test file but never uses it in production code.

**Recommendation**: Accept interfaces covering just the methods each consumer needs.

#### H7. EDI `Backend` Interface Is Incomplete

**File**: `edi/internal/recall/backend.go:14-19`

```go
type Backend interface {
    Search(query string, types []string, scope string, limit int) ([]Item, error)
    Add(item *Item) error
    FindByTitle(title string) (*Item, error)
    Close() error
}
```

The `Server` calls `s.storage.Get()`, `s.storage.RecordFeedback()`, `s.storage.LogFlightRecorder()` (lines 404, 472, 502) which are **not on the `Backend` interface**. This makes `Backend` an incomplete abstraction.

**Recommendation**: Add the missing methods, or create a `ServerBackend` that extends `Backend`.

#### H8. `MetadataStorage` Interface Is Too Fat

**File**: `codex/internal/core/interfaces.go:42-53`

`MetadataStorage` has 9 methods spanning three concerns: item CRUD, feedback, and flight recorder. Per the Interface Segregation Principle, split into:

```go
type ItemStorage interface { SaveItem, GetItem, FindByTitle, ListItems, DeleteItem, CountItemsByType, Close }
type FeedbackRecorder interface { RecordFeedback }
type FlightRecorder interface { LogFlightRecorder, GetFlightRecorderEntries }
```

#### H9. Swallowed `os.UserHomeDir()` Errors (20+ locations)

**Files**: Throughout `edi/internal/cli/`, `edi/internal/config/`, `edi/internal/launch/`, `edi/internal/tasks/`

```go
home, _ := os.UserHomeDir()  // error silently ignored
```

Found in 20+ locations. If `os.UserHomeDir()` fails, `home` is `""`, producing paths like `"/.edi/config.yaml"` — valid but wrong.

Key locations:
- `edi/internal/config/loader.go:62-83` (4 exported helper functions)
- `edi/internal/cli/recall.go:94, 198, 221`
- `edi/internal/cli/launch.go:70`
- `edi/internal/launch/mcp.go:37, 53, 89, 194, 227`
- `edi/internal/tasks/sync.go:249`

**Recommendation**: Resolve HOME once at program start and pass through, or create a `mustUserHomeDir()` helper.

#### H10. Variable Shadowing: `ctx` Shadows `context.Context` Convention

**Files**: `edi/internal/recall/server.go:462`, `codex/internal/mcp/tools.go:233`

```go
ctx, _ := args["context"].(string)  // 'ctx' universally means context.Context
```

**Recommendation**: Rename to `feedbackCtx` or `contextStr`.

### Medium Priority

#### M1. Temp Directory and Engine Leak in Eval Runner

**File**: `codex/eval/runner_agent.go:158-186`

`bootMCP` creates a temp directory at line 159 but only cleans it up on error paths (lines 174, 181). On success, the temp directory leaks. Additionally, the `engine` created at line 170 is never closed — `MCPClient.Close()` only cancels context and closes the pipe writer.

**Recommendation**: Return cleanup function or have `MCPClient` own both engine and tmpDir.

#### M2. Duplicated Helper Functions Across Modules

Multiple utility functions are copied rather than shared:

| Function | Location 1 | Location 2 | Difference |
|----------|-----------|-----------|------------|
| `getEnv(key, default)` | `codex/cmd/codex-cli/config.go:46` | `codex/cmd/recall-mcp/main.go:65` | Identical |
| `expandHome(path)` | `codex/internal/storage/metadata.go:63` | `codex/pkg/recall/client.go:292` | Subtly different (`"~"` vs `"~/"`) |
| `loadProfile(projectPath)` | `edi/internal/briefing/generator.go:165` | `edi/internal/memory/generator.go:324` | Identical |
| `loadStatus(projectPath)` | `edi/internal/briefing/generator.go:174` | `edi/internal/memory/generator.go:332` | Identical |
| `copyFile(src, dst)` | `edi/internal/cli/ralph.go:182` | `edi/internal/launch/commands.go:106` | **Different error handling** for `Close()` |
| frontmatter parsing | `edi/internal/agents/loader.go:67-116` | `edi/internal/briefing/history.go:70-119` | Nearly identical |
| `maxContentSize`/`maxQuerySize` | `codex/internal/mcp/tools.go:46` | `codex/internal/web/handlers.go:16` | Identical constants |

The `copyFile` duplication is particularly dangerous: `launch/commands.go` correctly captures `Close()` errors via named returns; `ralph.go` does not.

**Recommendation**: Extract shared utilities into `internal/pathutil`, `internal/fileutil`, or similar packages.

#### M3. Duplicated Row Scanning in EDI Storage

**File**: `edi/internal/recall/storage.go`

The same scan-and-parse pattern (scan columns, parse tags JSON, parse timestamps) appears identically in `Search` (lines 126-163), `Get` (lines 236-267), and `FindByTitle` (lines 174-201).

**Recommendation**: Extract a `scanItem(scanner) (*Item, error)` helper.

#### M4. Duplicated Retry Logic and Code Fence Stripping in Eval

**Files**: `codex/eval/judge.go:93-144` vs `judge.go:163-206`, `judge.go:212-223` vs `scorer.go:394-403`

The retry-with-exponential-backoff pattern is nearly identical in `RawJudge` and `RawHTTPPost`. The markdown code fence stripping logic is duplicated in `parseJudgment` and `judgeCodeQuality`. Both should be extracted to shared helpers.

#### M5. Hand-Rolled String Functions in Eval

**File**: `codex/eval/runner_agent.go:507-541`

`containsIgnoreCase` and `bytesContains` are hand-rolled ASCII-only implementations that ignore Unicode.

**Fix**: `strings.Contains(strings.ToLower(s), strings.ToLower(substr))`

**File**: `codex/eval/runner_pipe.go:306-319` — `splitLines` reimplements `strings.Split(s, "\n")`.

#### M6. Reinvented Stdlib Functions

**File**: `edi/internal/recall/integration_test.go:666-669`

```go
func hasPrefix(s, prefix string) bool {
    return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
```

This is exactly `strings.HasPrefix`. Delete and use stdlib.

#### M7. `interface{}` vs `any` Inconsistency

The project requires Go 1.22+ but mixes both forms:

- Uses `any`: `codex/internal/core/types.go`, `codex/internal/storage/metadata.go`, `codex/internal/storage/vecstore.go`
- Uses `interface{}`: `codex/internal/mcp/server.go`, `edi/internal/recall/server.go`, `edi/internal/tasks/manifest.go`, most of `codex/internal/mcp/tools.go`

**Recommendation**: Standardize on `any` throughout.

#### M8. Error Message Convention Violations

Multiple locations violate Go Code Review Comments ("Error strings should not be capitalized"):

```go
// codex/internal/core/index.go:65-75
return nil, fmt.Errorf("Embedder is required")
return nil, fmt.Errorf("VectorStore is required")
return nil, fmt.Errorf("MetaStore is required")
return nil, fmt.Errorf("CodeChunker is required")
```

Also: `fmt.Errorf` used without format arguments where `errors.New` is appropriate:
```go
// codex/internal/mcp/tools.go:54
return nil, fmt.Errorf("query is required")  // should be errors.New
```

#### M9. Magic Numbers Throughout

| File | Line | Value | Meaning |
|------|------|-------|---------|
| `codex/internal/core/engine.go` | 116 | `50` | candidate limit |
| `codex/internal/core/engine.go` | 161 | `60` | RRF k parameter |
| `codex/internal/mcp/server.go` | 101 | `10<<20` | stdin buffer |
| `edi/internal/recall/server.go` | 99 | `10<<20` | stdin buffer |
| `edi/internal/recall/server.go` | 120,147,307,383 | `-32700,-32601,-32602,100` | JSON-RPC codes, limits |
| `codex/eval/scorer.go` | 367 | `50000` | truncation limit |
| `codex/eval/agent_tools.go` | 246,367 | `100000,10000` | truncation limits |

**Recommendation**: Extract to named constants with documentation.

#### M10. Inconsistent Logging Strategy

Three different logging approaches:

1. `log.Printf` (standard library) — majority of codebase, `codex/internal/core/`, `edi/internal/recall/storage.go`
2. `log/slog` (structured logging) — `codex/internal/web/handlers.go`, `codex/pkg/recall/client.go`
3. `fmt.Fprintf(os.Stderr, ...)` — `edi/internal/cli/` package

**Recommendation**: Standardize on `log/slog` (available since Go 1.21, within the 1.22+ requirement).

#### M11. Compensating Actions Without Transactions

**File**: `codex/internal/core/engine.go:237-256`

```go
if err := e.metadata.SaveItem(itemToRecord(item)); err != nil {
    return fmt.Errorf("failed to save metadata: %w", err)
}
if err := e.vecStore.Upsert(ctx, item.ID, vec); err != nil {
    _ = e.metadata.DeleteItem(item.ID)  // compensating action, error ignored
    return fmt.Errorf("failed to store vector: %w", err)
}
```

If `DeleteItem` fails, the database is in an inconsistent state. Since both stores share the same SQLite database, consider wrapping in a SQL transaction.

#### M12. Repeated `os.Getwd()` Calls in Same Flow

During a single launch, `os.Getwd()` is called independently in:
- `edi/internal/cli/launch.go:25`
- `edi/internal/briefing/generator.go:43`
- `edi/internal/config/loader.go:22`
- `edi/internal/agents/loader.go:30`

**Recommendation**: Resolve `cwd` once at the top of the launch flow and pass as a parameter. Functions that call `os.Getwd()` internally are harder to test (require `os.Chdir`).

#### M13. `MCPClient` Reader Not Protected by Mutex

**File**: `codex/eval/mcpclient.go:105-127`

The `mu` mutex only protects writes. If multiple goroutines call `call()` concurrently, read interleaving is possible (goroutine A sends request, goroutine B sends request, goroutine A reads goroutine B's response). Currently single-threaded, but a latent issue.

#### M14. `dstFile.Close()` Error Not Checked

**File**: `edi/internal/codex/installer.go:101-108`

```go
defer dstFile.Close()  // error silently discarded
```

On some filesystems (NFS), `Close()` is where write errors surface. The `launch/commands.go` version of `copyFile` (lines 106-124) correctly captures the close error, but this version does not.

### Low Priority

#### L1. Missing Doc Comments on Exported MCP Types

**Files**: `edi/internal/recall/server.go:30-95`, `codex/internal/mcp/server.go:30-91`

13+ exported types (`MCPRequest`, `MCPResponse`, `MCPError`, `Tool`, `CallToolParams`, etc.) lack doc comments. Per Effective Go: "Every exported name should have a doc comment."

#### L2. Stale TODO Comments

**File**: `codex/internal/core/migrate.go:162-173` — Comments reference "Voyage embeddings" and "OpenAI embeddings" but the system uses local Ollama nomic-embed-text for all types.

**File**: `codex/internal/reranking/reranker.go` — Lines 11, 40, 71, 107, 124, 143 all contain `TODO: Implement with Hugot` without tracking issues.

#### L3. `ClaudeTask.MarshalJSON` Is Redundant

**File**: `edi/internal/tasks/manifest.go:82-85`

```go
func (ct *ClaudeTask) MarshalJSON() ([]byte, error) {
    type Alias ClaudeTask
    return json.Marshal((*Alias)(ct))
}
```

This custom `MarshalJSON` delegates to default behavior via alias — it does nothing `json.Marshal` wouldn't already do. Remove it.

#### L4. `ASTChunker.available` Field Is Always True

**File**: `codex/internal/chunking/ast.go:22,27`

The `available` field is always set to `true` and never changes. `IsAvailable()` always returns `true`. Dead code from a feature flag that was never needed.

#### L5. `DetectLanguage` Lowercases Entire Path

**File**: `codex/internal/chunking/ast.go:388`

```go
ext := strings.ToLower(filePath)  // lowercases entire path
```

Should use `strings.ToLower(filepath.Ext(filePath))` — only the extension matters.

#### L6. No `.golangci.yml` Configuration

The Makefiles reference `golangci-lint` but no configuration file exists. Running with defaults may miss project-specific issues.

**Recommendation**: Add `.golangci.yml` with `errcheck`, `govet`, `staticcheck`, `gosimple`, `gofumpt`, `contextcheck`.

#### L7. Unsafe Type Assertions in Integration Tests

**File**: `edi/internal/recall/integration_test.go:163, 188, 211, 481, 503, 525, 594`

```go
count := result["count"].(float64)             // panics if missing
results := result["results"].([]interface{})   // panics if wrong type
```

These produce confusing stack traces instead of clear test failures. Use comma-ok pattern with `t.Fatalf`.

#### L8. Duplicated Extension Maps in Indexer

**File**: `codex/internal/core/index.go:395-431`

`detectContentType` and `isIndexable` both define maps of file extensions with significant overlap. Extract to a shared `extensionInfo` map.

#### L9. `collectGoFiles` Reads All Files Into Memory

**File**: `codex/eval/scorer.go:309-324`

Loads all Go source files into memory simultaneously and is called twice during scoring. Should call once and pass result, and use `filepath.WalkDir` (more efficient).

#### L10. Global Manifest Lock

**File**: `edi/internal/tasks/sync.go:16`

```go
var manifestLock sync.Mutex
```

Package-level mutex makes independent manifest operations in tests impossible and contends on same lock for different project paths.

#### L11. Type Assertion Breaks Abstraction

**File**: `codex/pkg/recall/client.go:222-226`

```go
ms, ok := c.engine.MetadataStore().(*storage.MetadataStore)
```

`MetadataStore()` returns `MetadataStorage` (interface), but caller immediately asserts back to concrete type. Defeats the purpose of the interface.

#### L12. `Metadata` Type Alias Is Ambiguous

**File**: `edi/internal/tasks/manifest.go:33`

```go
type Metadata map[string]interface{}
```

Named type for `map[string]interface{}` without methods or validation. Consider using `map[string]any` inline.

---

## Positive Patterns Worth Preserving

1. **Interface segregation in `core/interfaces.go`** — small, focused interfaces (2-3 methods each)
2. **Error wrapping with `%w`** — maintains error chains for debugging
3. **Functional options pattern** in `embedding.LocalClient`, `web.ServerOption`
4. **Compile-time interface checks** — `var _ Backend = (*Storage)(nil)`
5. **Min-heap for top-K search** in `vecstore.go` — algorithmically optimal
6. **BDD-style test naming** — descriptive and self-documenting
7. **`doc.go` files** in packages — exemplary package-level documentation
8. **Build tag separation** (`fts5`, `evalintegration`) — clean build boundaries
9. **Atomic file writes** via temp + rename in `tasks/sync.go`
10. **Proper transaction pattern** — `defer tx.Rollback()` in storage (no-op after commit)
11. **Exponential backoff retry** in `embedding/local.go` and `eval/judge.go`
12. **Named return + deferred closure** for `Close()` error capture in `launch/commands.go`
13. **Per-language parser mutexes** in `ASTChunker` — allows concurrent parsing of different languages
14. **Signal handling** in CLI — proper graceful shutdown via signal channels

---

## Recommended Priority Actions

| Priority | Action | Effort | Impact |
|----------|--------|--------|--------|
| **1** | Fix slice corruption bug in `eval/condition.go:124` | Low | Critical — data corruption |
| **2** | Fix MCP server context cancellation (both modules) | Medium | Critical — affects shutdown |
| **3** | Replace `err ==` with `errors.Is` (6 locations) | Low | High — correctness |
| **4** | Add MCP server unit tests | Medium | High — zero coverage on primary interface |
| **5** | Extract shared MCP types to eliminate duplication | Medium | High — maintenance |
| **6** | Make MCP Server/ToolHandler accept interfaces | Medium | High — testability |
| **7** | Complete EDI `Backend` interface | Low | High — abstraction correctness |
| **8** | Define sentinel errors for not-found | Low | High — programmatic error handling |
| **9** | Fix temp dir + engine leak in `eval/runner_agent.go` | Low | High — resource leak |
| **10** | Handle `os.UserHomeDir()` errors consistently | Medium | High — 20+ silent failures |
| **11** | Extract duplicated helpers (6 function pairs) | Medium | Medium — maintenance |
| **12** | Standardize on `any` over `interface{}` | Low | Medium — consistency |
| **13** | Standardize on `log/slog` | Medium | Medium — observability |
| **14** | Add `.golangci.yml` | Low | Medium — automated enforcement |
| **15** | Replace magic numbers with named constants | Low | Low — readability |
