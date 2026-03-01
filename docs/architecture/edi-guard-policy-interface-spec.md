# ADR: edi-guard Policy Interface Refactor

| Field   | Value |
|---------|-------|
| Status  | Proposed |
| Date    | 2026-03-01 |
| Authors | AI Engineering Framework Team |
| Scope   | `edi/cmd/edi-guard/`, `edi/internal/guard/` |

## Context

`edi-guard` currently implements four hardcoded policies in a single 519-line
`main.go`. Each policy is a set of functions called directly from event
handlers (`handlePreToolUse`, `handlePostToolUse`, etc.) with no shared
abstraction.

This works at four policies. It won't work at ten. The problems:

1. **Adding a new policy touches the event dispatch switch.** Every new policy
   requires editing `handlePreToolUse()` directly, weaving new logic between
   existing checks. There's no way to add a policy without understanding all
   existing policies.

2. **No isolation.** Build tag injection, deny-list checking, and failure
   counting all share the same function scope. A bug in one can silently break
   the response for another.

3. **Testing couples to internals.** Tests call `injectBuildTags()` and
   `checkDenyList()` directly. Adding a policy means adding more exported
   functions to `main.go` — there's no consistent contract.

4. **Config is policy-specific but loaded generically.** `GuardConfig` has
   fields for every policy (`BuildTags`, `DenyPatterns`, `FailureLoopThreshold`)
   with no way for a policy to declare what config it needs.

### What works today and must not break

- Deny patterns block commands (exit 2) with reasons on stderr
- Build tags are injected transparently via `updatedInput`
- Failure loop advisory fires after N consecutive failures
- PreCompact writes session state to `/memories/`
- Config merges global + project YAML with correct semantics
- `.edi/` directory check gates all processing
- 25+ existing tests pass without modification
- Binary has zero CGO dependency

## Decision

Introduce a `Policy` interface in a new `edi/internal/guard/` package. Extract
each existing policy into its own type. The `main.go` event dispatcher becomes
a thin loop over registered policies.

### Design constraints

1. **No overengineering.** The interface must have one method per hook event,
   not a generic `Evaluate()` that requires each policy to understand response
   merging. Policies declare which events they care about by implementing
   optional interfaces.

2. **Ordering matters.** Deny-list must run before build-tag injection (a
   blocked command should never be modified). The registry is an ordered slice,
   not a map.

3. **Short-circuit on block.** If any policy returns `block`, stop evaluating
   further policies. This preserves the current behavior where deny-list exit 2
   prevents build tag injection and failure advisory.

4. **Response merging is the dispatcher's job.** Policies return their
   individual effects (`updatedInput`, `additionalContext`, `block`). The
   dispatcher merges them into a single JSON response. Policies never construct
   the full hook output JSON.

5. **Config stays in `config.GuardConfig`.** Policies receive the full
   `GuardConfig` at construction time. We don't need per-policy config
   registration — that's premature for <10 policies.

---

## Specification

### 1. Policy Interface

In `edi/internal/guard/policy.go`:

```go
package guard

import "context"

// PolicyResult is returned by policy evaluation methods.
type PolicyResult struct {
    // Block stops the tool call. When true, Reason is written to stderr
    // and the hook exits with code 2. Only meaningful for PreToolUse.
    Block  bool
    Reason string

    // ModifiedCommand replaces tool_input.command when non-empty.
    // Multiple policies may modify the command; they are applied in order.
    // Only meaningful for PreToolUse.
    ModifiedCommand string

    // Advisory is injected as additionalContext.
    // Multiple advisories from different policies are joined with newlines.
    // Only meaningful for PreToolUse.
    Advisory string
}

// HookContext carries the parsed hook input and resolved config.
type HookContext struct {
    Input     *HookInput
    Config    *GuardConfig
    SessionID string
    CWD       string
}

// PreToolUsePolicy is implemented by policies that evaluate before tool execution.
type PreToolUsePolicy interface {
    // Name returns a short identifier for logging/debugging (e.g., "deny-list").
    Name() string

    // EvalPreToolUse evaluates the policy for a PreToolUse event.
    // command is the current command (may have been modified by an earlier policy).
    // Returns a result. A nil result means "no opinion."
    EvalPreToolUse(ctx context.Context, hctx *HookContext, command string) *PolicyResult
}

// PostToolUsePolicy is implemented by policies that react to successful tool completion.
type PostToolUsePolicy interface {
    Name() string
    OnPostToolUse(ctx context.Context, hctx *HookContext)
}

// PostToolUseFailurePolicy is implemented by policies that react to tool failures.
type PostToolUseFailurePolicy interface {
    Name() string
    OnPostToolUseFailure(ctx context.Context, hctx *HookContext)
}

// PreCompactPolicy is implemented by policies that act before context compaction.
type PreCompactPolicy interface {
    Name() string
    OnPreCompact(ctx context.Context, hctx *HookContext)
}
```

### Why four separate interfaces instead of one

A policy that only cares about PreToolUse (deny-list) should not need stub
methods for PostToolUse, PostToolUseFailure, and PreCompact. Go's implicit
interface satisfaction means a type only needs to implement the methods it
actually provides. The dispatcher checks `if p, ok := policy.(PreToolUsePolicy)`.

This is the standard Go idiom (see `io.Reader`/`io.Writer`/`io.Closer` vs a
single `io.ReadWriteCloser`). It avoids the `// not implemented` stubs that
plague large interfaces.

### 2. Policy Registry

In `edi/internal/guard/registry.go`:

```go
package guard

// Registry holds an ordered list of policies.
// Order matters: policies are evaluated first-to-last, and a Block
// from any policy short-circuits further evaluation.
type Registry struct {
    policies []interface{} // each implements one or more Policy interfaces
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
    return &Registry{}
}

// Register adds a policy to the end of the evaluation order.
func (r *Registry) Register(policy interface{}) {
    r.policies = append(r.policies, policy)
}

// PreToolUsePolicies returns all policies that implement PreToolUsePolicy.
func (r *Registry) PreToolUsePolicies() []PreToolUsePolicy {
    var result []PreToolUsePolicy
    for _, p := range r.policies {
        if ptu, ok := p.(PreToolUsePolicy); ok {
            result = append(result, ptu)
        }
    }
    return result
}

// PostToolUsePolicies returns all policies that implement PostToolUsePolicy.
func (r *Registry) PostToolUsePolicies() []PostToolUsePolicy {
    var result []PostToolUsePolicy
    for _, p := range r.policies {
        if pp, ok := p.(PostToolUsePolicy); ok {
            result = append(result, pp)
        }
    }
    return result
}

// PostToolUseFailurePolicies returns all policies that implement PostToolUseFailurePolicy.
func (r *Registry) PostToolUseFailurePolicies() []PostToolUseFailurePolicy {
    var result []PostToolUseFailurePolicy
    for _, p := range r.policies {
        if pp, ok := p.(PostToolUseFailurePolicy); ok {
            result = append(result, pp)
        }
    }
    return result
}

// PreCompactPolicies returns all policies that implement PreCompactPolicy.
func (r *Registry) PreCompactPolicies() []PreCompactPolicy {
    var result []PreCompactPolicy
    for _, p := range r.policies {
        if pp, ok := p.(PreCompactPolicy); ok {
            result = append(result, pp)
        }
    }
    return result
}
```

### 3. Default Registration

In `edi/internal/guard/defaults.go`:

```go
package guard

// DefaultRegistry creates a registry with all built-in policies in the
// correct evaluation order.
func DefaultRegistry(cfg *GuardConfig) *Registry {
    r := NewRegistry()

    // Order matters:
    // 1. Deny-list first (blocking — short-circuits everything else)
    // 2. Build tags (modifies command — must run before advisory reads it)
    // 3. Failure loop (reads command, may add advisory)
    // 4. Compaction snapshot (only fires on PreCompact, ordering is irrelevant)
    r.Register(NewDenyListPolicy(cfg.DenyPatterns))
    r.Register(NewBuildTagPolicy(cfg.BuildTags))
    r.Register(NewFailureLoopPolicy(cfg.FailureLoopThreshold))
    r.Register(NewCompactionSnapshotPolicy(cfg))

    return r
}
```

### 4. Existing Policies as Types

Each policy becomes a self-contained type in `edi/internal/guard/`. The logic
is extracted verbatim from `main.go` — no behavioral changes.

#### 4.1 DenyListPolicy

File: `edi/internal/guard/policy_denylist.go`

```go
package guard

import (
    "context"
    "regexp"
)

type DenyListPolicy struct {
    patterns []compiledPattern
}

type compiledPattern struct {
    re     *regexp.Regexp
    reason string
}

func NewDenyListPolicy(patterns []DenyPattern) *DenyListPolicy {
    compiled := make([]compiledPattern, 0, len(patterns))
    for _, p := range patterns {
        re, err := regexp.Compile(p.Pattern)
        if err != nil {
            continue // skip invalid patterns, log to stderr
        }
        compiled = append(compiled, compiledPattern{re: re, reason: p.Reason})
    }
    return &DenyListPolicy{patterns: compiled}
}

func (d *DenyListPolicy) Name() string { return "deny-list" }

func (d *DenyListPolicy) EvalPreToolUse(_ context.Context, _ *HookContext, command string) *PolicyResult {
    for _, p := range d.patterns {
        if p.re.MatchString(command) {
            return &PolicyResult{Block: true, Reason: p.reason}
        }
    }
    return nil
}
```

Implements: `PreToolUsePolicy` only.

#### 4.2 BuildTagPolicy

File: `edi/internal/guard/policy_buildtag.go`

```go
type BuildTagPolicy struct {
    tags []string
}

func NewBuildTagPolicy(tags []string) *BuildTagPolicy {
    return &BuildTagPolicy{tags: tags}
}

func (b *BuildTagPolicy) Name() string { return "build-tags" }

func (b *BuildTagPolicy) EvalPreToolUse(_ context.Context, _ *HookContext, command string) *PolicyResult {
    if len(b.tags) == 0 {
        return nil
    }
    modified, newCommand := injectBuildTags(command, b.tags)
    if !modified {
        return nil
    }
    return &PolicyResult{ModifiedCommand: newCommand}
}
```

Implements: `PreToolUsePolicy` only.

The `injectBuildTags`, `hasAllTags`, `injectTagsIntoClause` helper functions
move to this file as unexported functions within the `guard` package.

#### 4.3 FailureLoopPolicy

File: `edi/internal/guard/policy_failureloop.go`

```go
type FailureLoopPolicy struct {
    threshold int
}

func NewFailureLoopPolicy(threshold int) *FailureLoopPolicy {
    if threshold <= 0 {
        threshold = 5
    }
    return &FailureLoopPolicy{threshold: threshold}
}

func (f *FailureLoopPolicy) Name() string { return "failure-loop" }

func (f *FailureLoopPolicy) EvalPreToolUse(_ context.Context, hctx *HookContext, _ string) *PolicyResult {
    state := readState(hctx.SessionID)
    if state.ConsecutiveFailures < f.threshold || state.Advised {
        return nil
    }
    advisory := fmt.Sprintf(
        "edi-guard: %d consecutive Bash command failures detected. "+
            "The last failure was: %q → %q. "+
            "Consider stepping back to analyze the root cause rather than retrying with small variations.",
        state.ConsecutiveFailures, state.LastFailureCommand, state.LastFailureError,
    )
    state.Advised = true
    writeState(hctx.SessionID, state)
    return &PolicyResult{Advisory: advisory}
}

func (f *FailureLoopPolicy) OnPostToolUse(_ context.Context, hctx *HookContext) {
    state := readState(hctx.SessionID)
    if state.ConsecutiveFailures == 0 && !state.Advised {
        return
    }
    writeState(hctx.SessionID, guardState{})
}

func (f *FailureLoopPolicy) OnPostToolUseFailure(_ context.Context, hctx *HookContext) {
    bash := parseBashInput(hctx.Input.ToolInput)
    state := readState(hctx.SessionID)
    state.ConsecutiveFailures++
    state.Advised = false
    if bash != nil {
        state.LastFailureCommand = bash.Command
    }
    state.LastFailureError = hctx.Input.Error
    writeState(hctx.SessionID, state)
}
```

Implements: `PreToolUsePolicy`, `PostToolUsePolicy`, `PostToolUseFailurePolicy`.

#### 4.4 CompactionSnapshotPolicy

File: `edi/internal/guard/policy_compaction.go`

```go
type CompactionSnapshotPolicy struct {
    cfg *GuardConfig
}

func NewCompactionSnapshotPolicy(cfg *GuardConfig) *CompactionSnapshotPolicy {
    return &CompactionSnapshotPolicy{cfg: cfg}
}

func (c *CompactionSnapshotPolicy) Name() string { return "compaction-snapshot" }

func (c *CompactionSnapshotPolicy) OnPreCompact(_ context.Context, hctx *HookContext) {
    // Existing handlePreCompact logic extracted verbatim
}
```

Implements: `PreCompactPolicy` only.

### 5. Dispatcher (new main.go)

The refactored `main.go` becomes a thin dispatcher:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/anthropics/aef/edi/internal/guard"
)

func main() {
    input := guard.ParseStdin()
    if input == nil {
        os.Exit(0)
    }

    if _, err := os.Stat(filepath.Join(input.CWD, ".edi")); os.IsNotExist(err) {
        os.Exit(0)
    }

    cfg := guard.LoadConfig(input.CWD)
    if !cfg.Guard.Enabled {
        os.Exit(0)
    }

    registry := guard.DefaultRegistry(&cfg.Guard)
    hctx := &guard.HookContext{
        Input:     input,
        Config:    &cfg.Guard,
        SessionID: input.SessionID,
        CWD:       input.CWD,
    }
    ctx := context.Background()

    switch input.HookEventName {
    case "PreToolUse":
        dispatchPreToolUse(ctx, registry, hctx)
    case "PostToolUse":
        dispatchPostToolUse(ctx, registry, hctx)
    case "PostToolUseFailure":
        dispatchPostToolUseFailure(ctx, registry, hctx)
    case "PreCompact":
        dispatchPreCompact(ctx, registry, hctx)
    }
}

func dispatchPreToolUse(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
    if hctx.Input.ToolName != "Bash" {
        return
    }
    bash := guard.ParseBashInput(hctx.Input.ToolInput)
    if bash == nil || bash.Command == "" {
        return
    }

    command := bash.Command
    var advisories []string
    modified := false

    for _, policy := range reg.PreToolUsePolicies() {
        result := policy.EvalPreToolUse(ctx, hctx, command)
        if result == nil {
            continue
        }

        // Block short-circuits immediately
        if result.Block {
            fmt.Fprintf(os.Stderr, "edi-guard [%s]: %s\n", policy.Name(), result.Reason)
            os.Exit(2)
        }

        // Apply command modification (chained — each policy sees previous modifications)
        if result.ModifiedCommand != "" {
            command = result.ModifiedCommand
            modified = true
        }

        // Collect advisories
        if result.Advisory != "" {
            advisories = append(advisories, result.Advisory)
        }
    }

    // Build response if anything changed
    if modified || len(advisories) > 0 {
        resp := guard.BuildPreToolUseResponse(command, advisories, modified)
        data, _ := json.Marshal(resp)
        fmt.Println(string(data))
    }
}

func dispatchPostToolUse(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
    if hctx.Input.ToolName != "Bash" {
        return
    }
    for _, policy := range reg.PostToolUsePolicies() {
        policy.OnPostToolUse(ctx, hctx)
    }
}

func dispatchPostToolUseFailure(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
    if hctx.Input.ToolName != "Bash" {
        return
    }
    for _, policy := range reg.PostToolUseFailurePolicies() {
        policy.OnPostToolUseFailure(ctx, hctx)
    }
}

func dispatchPreCompact(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
    for _, policy := range reg.PreCompactPolicies() {
        policy.OnPreCompact(ctx, hctx)
    }
}
```

### 6. Package Layout

```
edi/internal/guard/
├── policy.go                  # Interfaces: PreToolUsePolicy, PostToolUsePolicy, etc.
├── registry.go                # Registry type + accessor methods
├── defaults.go                # DefaultRegistry() constructor
├── types.go                   # HookInput, HookContext, GuardConfig re-export, etc.
├── state.go                   # readState/writeState (shared by failure loop)
├── response.go                # BuildPreToolUseResponse, ParseStdin, ParseBashInput
├── policy_denylist.go         # DenyListPolicy
├── policy_denylist_test.go    # Deny list tests (moved from main_test.go)
├── policy_buildtag.go         # BuildTagPolicy + injectBuildTags helpers
├── policy_buildtag_test.go    # Build tag tests
├── policy_failureloop.go      # FailureLoopPolicy
├── policy_failureloop_test.go # Failure counter tests
├── policy_compaction.go       # CompactionSnapshotPolicy
├── policy_compaction_test.go  # PreCompact tests
├── registry_test.go           # Registry ordering, short-circuit, merging tests
└── dispatch_test.go           # Integration: full PreToolUse dispatch scenarios

edi/cmd/edi-guard/
├── main.go                    # Thin dispatcher (~80 lines)
└── main_test.go               # Config loading tests (remain here — test main package)
```

### 7. Response Merging Rules

The dispatcher merges `PolicyResult` values from multiple policies. The rules:

| Field | Merge Strategy | Rationale |
|-------|---------------|-----------|
| `Block` | First block wins, short-circuits | Deny-list stops everything |
| `ModifiedCommand` | Chain — each policy receives previous policy's output | Build tags → future "normalize paths" policy |
| `Advisory` | Join with `\n` | Multiple policies may have different warnings |

This means `EvalPreToolUse` receives the **current** command string (possibly
already modified by an earlier policy), not the original. This is essential for
policy chaining — a future "path normalization" policy could clean up a command
before the build-tag policy inspects it.

### 8. What Adding a New Policy Looks Like

Example: a "warn on untested changes" policy that detects `git commit` when
tests haven't been run in the current session.

```go
// edi/internal/guard/policy_commitguard.go
package guard

import "context"

type CommitGuardPolicy struct{}

func NewCommitGuardPolicy() *CommitGuardPolicy {
    return &CommitGuardPolicy{}
}

func (c *CommitGuardPolicy) Name() string { return "commit-guard" }

func (c *CommitGuardPolicy) EvalPreToolUse(_ context.Context, hctx *HookContext, command string) *PolicyResult {
    if !strings.Contains(command, "git commit") {
        return nil
    }
    state := readState(hctx.SessionID)
    if !state.TestsRanThisSession {
        return &PolicyResult{
            Advisory: "edi-guard: committing without running tests this session. Consider running tests first.",
        }
    }
    return nil
}
```

To activate: one line in `defaults.go`:

```go
r.Register(NewCommitGuardPolicy())
```

No changes to `main.go`, no changes to dispatch logic, no changes to response
building. The new policy is fully self-contained.

### 9. What Stays in `main.go`

- `main()` function with stdin parsing, `.edi/` check, config loading
- The four `dispatch*` functions (~60 lines total)
- Config loading: `loadGuardConfig()` and `mergeGuardOverlay()` — these are
  binary-specific (reads YAML from disk with specific merge rules). They stay
  in `main.go` but call into `guard.LoadConfig()` if we want to share, or
  remain as-is. The decision: **keep config loading in main.go** since it
  involves file I/O paths that are specific to the binary's execution context.

### 10. Migration Strategy

This is a refactor, not a rewrite. The migration preserves exact behavior:

**Step 1: Create `edi/internal/guard/` with interfaces and registry.**
No behavior change. Just new files.

**Step 2: Extract each policy into its own file.**
Move code from `main.go` into `guard/policy_*.go`. Each extraction is a pure
code move — the logic doesn't change.

**Step 3: Move tests alongside policies.**
Split `main_test.go` into `policy_*_test.go` files. Tests call the same
functions but through the policy interface.

**Step 4: Rewrite `main.go` as thin dispatcher.**
Replace the four `handle*` functions with the `dispatch*` functions that loop
over the registry.

**Step 5: Add `registry_test.go` with integration tests.**
Test ordering, short-circuit, and response merging as new behaviors.

Each step is a separate commit. At every step, `go test ./edi/cmd/edi-guard/...`
and `go test ./edi/internal/guard/...` must pass.

## Testing

### Preserved tests (moved, not rewritten)

All 25 existing tests in `main_test.go` move to the appropriate
`policy_*_test.go` files. They call the same underlying functions through
the policy types instead of free functions. No assertion changes.

### New tests

| Test | File | Purpose |
|------|------|---------|
| `TestRegistry_OrderPreserved` | `registry_test.go` | Policies evaluate in registration order |
| `TestRegistry_BlockShortCircuits` | `registry_test.go` | After a Block, later policies don't run |
| `TestRegistry_CommandChaining` | `registry_test.go` | Policy B sees Policy A's modified command |
| `TestRegistry_AdvisoriesMerge` | `registry_test.go` | Two advisories are joined with newline |
| `TestRegistry_EmptyResultSkipped` | `registry_test.go` | Nil result from policy is ignored |
| `TestDispatch_PreToolUse_BashOnly` | `dispatch_test.go` | Non-Bash tools are ignored |
| `TestDispatch_PostToolUse_AllFire` | `dispatch_test.go` | All PostToolUse policies run |
| `TestDefaultRegistry_HasAllPolicies` | `defaults_test.go` | DefaultRegistry returns 4 policies |

### Test migration mapping

| Current test | Moves to |
|---|---|
| `TestInjectBuildTags_*` (10 tests) | `policy_buildtag_test.go` |
| `TestDenyList_*` (8 tests) | `policy_denylist_test.go` |
| `TestFailureCounter_*` (2 tests) | `policy_failureloop_test.go` |
| `TestPreCompact_*` (3 tests) | `policy_compaction_test.go` |
| `TestBuildPreToolUseResponse_*` (3 tests) | `response_test.go` |
| `TestHasAllTags_*` (4 tests) | `policy_buildtag_test.go` |
| `TestCompileDenyPatterns_*` (1 test) | `policy_denylist_test.go` |
| `TestLoadGuardConfig_*` (6 tests) | stay in `main_test.go` |

## Risks

### 1. Interface overhead for a CLI binary

`edi-guard` is a short-lived process (invoked per hook event, runs <10ms).
Interface dispatch adds negligible overhead. The registry construction
(`DefaultRegistry`) allocates 4 small structs — this is not a performance
concern.

### 2. Package split increases import surface

Moving from one `main` package to `main` + `guard` adds an import boundary.
This is desirable — `guard` is testable without building the binary, and
policies can be used in other contexts (e.g., a future `edi guard check`
CLI command).

### 3. Existing `config.GuardConfig` is used across packages

The `GuardConfig` and `DenyPattern` types stay in `edi/internal/config/` (where
they are today). The `guard` package imports them. No type duplication.

### 4. State file functions shared across policies

`readState()` and `writeState()` are used by `FailureLoopPolicy` and
`CompactionSnapshotPolicy`. They move to `guard/state.go` as package-level
functions, available to all policies.

## What This Does NOT Do

1. **No plugin system.** Policies are compiled into the binary. There's no
   dynamic loading, no `plugin.Open()`. For <20 policies this is the right
   choice — it keeps the binary simple and the type system intact.

2. **No per-policy config registration.** Policies receive the full
   `GuardConfig`. A policy reads the fields it needs and ignores the rest.
   If we reach 15+ policies with diverse config needs, we can add a
   `PolicyConfig interface{}` map. Not now.

3. **No policy-level enable/disable in config.** Today, you enable/disable
   guard globally. Individual policy toggle (`guard.policies.deny_list.enabled`)
   is a future enhancement if needed. The interface supports it (just don't
   register the policy), but we don't add config surface for it yet.

4. **No middleware/interceptor pattern.** Policies are not wrapping each other.
   They run sequentially with result merging. This is simpler and sufficient.

## Implementation Order

| Step | Description | Estimated |
|------|-------------|-----------|
| 1 | Create `guard/` package: interfaces, registry, types, response, state | ~1 hour |
| 2 | Extract `DenyListPolicy` + tests | ~30 min |
| 3 | Extract `BuildTagPolicy` + tests | ~30 min |
| 4 | Extract `FailureLoopPolicy` + tests | ~45 min |
| 5 | Extract `CompactionSnapshotPolicy` + tests | ~30 min |
| 6 | Write `DefaultRegistry` + `defaults_test.go` | ~15 min |
| 7 | Rewrite `main.go` as thin dispatcher | ~45 min |
| 8 | Add registry integration tests | ~30 min |
| 9 | Verify all 25+ existing tests pass | ~15 min |

**Total: ~5 hours** of focused work.
