# ADR: edi-guard Policy Interface Refactor

| Field   | Value |
|---------|-------|
| Status  | Proposed |
| Date    | 2026-03-01 |
| Updated | 2026-03-01 |
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
- 42 existing tests pass (with structural migration — see §Testing)
- Binary has zero CGO dependency
- stderr format: `edi-guard: <reason>` (preserved — see §Behavioral Notes)

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

6. **Bash-only scope.** edi-guard only guards Bash tool calls. The hook
   registration in `.claude/settings.json` uses `"Bash"` as the tool matcher
   for PreToolUse/PostToolUse/PostToolUseFailure events. The Bash check lives
   in the dispatcher, not in individual policies. Non-Bash policies would
   require hook registration changes in addition to dispatcher changes — this
   is an intentional constraint for now. (Finding 7)

7. **No panic recovery.** Policies do not have panic recovery. A panic in any
   policy crashes the hook. This is acceptable — `edi-guard` is a short-lived
   process (<10ms), not a long-running server. A panic is a programmer bug
   that should crash fast and surface a stack trace on stderr. Claude Code
   continues if the hook fails. (Finding 11)

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

// HookContext carries the parsed hook input and session metadata.
// Agent is session-level context (not part of GuardConfig) needed by
// policies like CompactionSnapshotPolicy that write session state.
type HookContext struct {
    Input     *HookInput
    Config    *config.GuardConfig
    SessionID string
    CWD       string
    Agent     string // (Finding 1) Session agent mode, e.g. "coder", "architect"
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

**Finding 1 resolution:** `Agent` is added to `HookContext` rather than
`GuardConfig`. The agent mode is session-level context (set at launch time),
not guard configuration. `main.go` already has it from `guardConfigFile.Agent`
and passes it when constructing `HookContext`. `CompactionSnapshotPolicy`
reads `hctx.Agent` instead of needing the full `guardConfigFile`.

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

import "fmt"

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
// Panics if the value implements none of the four policy interfaces.
// This is a programmer error caught at startup, not a runtime condition.
// (Finding 2)
func (r *Registry) Register(policy interface{}) {
    _, a := policy.(PreToolUsePolicy)
    _, b := policy.(PostToolUsePolicy)
    _, c := policy.(PostToolUseFailurePolicy)
    _, d := policy.(PreCompactPolicy)
    if !a && !b && !c && !d {
        panic(fmt.Sprintf("guard: registered policy %T implements none of "+
            "PreToolUsePolicy, PostToolUsePolicy, PostToolUseFailurePolicy, "+
            "PreCompactPolicy", policy))
    }
    r.policies = append(r.policies, policy)
}

// PreToolUsePolicies returns all policies that implement PreToolUsePolicy.
// Allocates a new slice on each call. Acceptable for <20 policies.
// (Finding 13: if this becomes a bottleneck, cache at construction time.)
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

import "github.com/anthropics/aef/edi/internal/config"

// DefaultRegistry creates a registry with all built-in policies in the
// correct evaluation order.
func DefaultRegistry(cfg *config.GuardConfig) *Registry {
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
    "fmt"
    "os"
    "regexp"

    "github.com/anthropics/aef/edi/internal/config"
)

type DenyListPolicy struct {
    patterns []compiledPattern
}

type compiledPattern struct {
    re     *regexp.Regexp
    reason string
}

func NewDenyListPolicy(patterns []config.DenyPattern) *DenyListPolicy {
    compiled := make([]compiledPattern, 0, len(patterns))
    for _, p := range patterns {
        re, err := regexp.Compile(p.Pattern)
        if err != nil {
            fmt.Fprintf(os.Stderr, "edi-guard: invalid deny pattern %q: %v\n", p.Pattern, err)
            continue
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
package guard

import "context"

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

The `injectBuildTags`, `hasAllTags`, `injectTagsIntoClause` helper functions,
and the `goCommandRe`/`clauseSplitRe` package-level regexps move to this file
as unexported symbols within the `guard` package.

#### 4.3 FailureLoopPolicy

File: `edi/internal/guard/policy_failureloop.go`

```go
package guard

import (
    "context"
    "fmt"
)

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
    bash := ParseBashInput(hctx.Input.ToolInput)
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

**Finding 6 resolution:** `ParseBashInput` is **exported** in
`guard/response.go`. It is called from both the dispatcher in `main.go` and
from `FailureLoopPolicy.OnPostToolUseFailure()`. The export is necessary
because `main.go` (package `main`) needs to call it to extract the Bash
command before the dispatch loop.

#### 4.4 CompactionSnapshotPolicy

File: `edi/internal/guard/policy_compaction.go`

```go
package guard

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/anthropics/aef/edi/internal/config"
    "gopkg.in/yaml.v3"
)

type CompactionSnapshotPolicy struct {
    cfg *config.GuardConfig
}

func NewCompactionSnapshotPolicy(cfg *config.GuardConfig) *CompactionSnapshotPolicy {
    return &CompactionSnapshotPolicy{cfg: cfg}
}

func (c *CompactionSnapshotPolicy) Name() string { return "compaction-snapshot" }

func (c *CompactionSnapshotPolicy) OnPreCompact(_ context.Context, hctx *HookContext) {
    var lines []string
    lines = append(lines, "# Session State (auto-generated by edi-guard before compaction)")

    // Tasks
    if tasks := readActiveTasks(hctx.CWD); len(tasks) > 0 {
        for i, t := range tasks {
            if i >= 2 {
                remaining := len(tasks) - 2
                lines[len(lines)-1] += fmt.Sprintf(" (+%d more)", remaining)
                break
            }
            lines = append(lines, fmt.Sprintf("- Task: %s — %s", t.id, t.subject))
        }
    }

    // Git branch
    if branch := gitBranch(); branch != "" {
        lines = append(lines, fmt.Sprintf("- Branch: %s", branch))
    }

    // Build tags
    if len(c.cfg.BuildTags) > 0 {
        lines = append(lines, fmt.Sprintf("- Build tags required: %s", strings.Join(c.cfg.BuildTags, ", ")))
    }

    // Last test result
    state := readState(hctx.SessionID)
    if state.ConsecutiveFailures > 0 {
        lines = append(lines, fmt.Sprintf("- Last test result: %d consecutive failures (last: %q → %q)",
            state.ConsecutiveFailures, state.LastFailureCommand, state.LastFailureError))
    } else {
        lines = append(lines, "- Last test result: no recent failures")
    }

    // Agent mode (Finding 1: read from HookContext, not GuardConfig)
    if hctx.Agent != "" {
        lines = append(lines, fmt.Sprintf("- Agent mode: %s", hctx.Agent))
    }

    // Compaction trigger
    if hctx.Input.Trigger != "" {
        lines = append(lines, fmt.Sprintf("- Compaction trigger: %s", hctx.Input.Trigger))
    }

    // Write file
    dir := filepath.Join(hctx.CWD, "memories")
    if err := os.MkdirAll(dir, 0755); err != nil {
        fmt.Fprintf(os.Stderr, "edi-guard: failed to create memories dir: %v\n", err)
        return
    }
    content := strings.Join(lines, "\n") + "\n"
    if err := os.WriteFile(filepath.Join(dir, "compaction-state.md"), []byte(content), 0600); err != nil {
        fmt.Fprintf(os.Stderr, "edi-guard: failed to write compaction-state.md: %v\n", err)
    }
}

// --- Unexported helpers (single-use within this policy) (Finding 9) ---

type taskInfo struct {
    id      string
    subject string
}

func readActiveTasks(cwd string) []taskInfo {
    data, err := os.ReadFile(filepath.Join(cwd, ".edi", "tasks", "active.yaml"))
    if err != nil {
        return nil
    }
    var manifest struct {
        Tasks []struct {
            ID      string `yaml:"id"`
            Subject string `yaml:"subject"`
            Status  string `yaml:"status"`
        } `yaml:"tasks"`
    }
    if err := yaml.Unmarshal(data, &manifest); err != nil {
        return nil
    }
    var result []taskInfo
    for _, t := range manifest.Tasks {
        if t.Status == "in_progress" {
            result = append(result, taskInfo{id: t.ID, subject: t.Subject})
        }
    }
    return result
}

func gitBranch() string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    out, err := exec.CommandContext(ctx, "git", "branch", "--show-current").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
```

Implements: `PreCompactPolicy` only.

**Finding 9 resolution:** `readActiveTasks()`, `gitBranch()`, and `taskInfo`
are single-use helpers that move to `policy_compaction.go` as unexported
symbols. They are self-contained within the compaction policy and have no
callers outside it.

### 5. Dispatcher (new main.go)

The refactored `main.go` becomes a thin dispatcher. Config loading stays in
`main.go` — it involves file I/O paths and YAML merge rules specific to the
binary's execution context (Finding 5).

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/anthropics/aef/edi/internal/config"
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

    cfg := loadGuardConfig(input.CWD)
    if !cfg.Guard.Enabled {
        os.Exit(0)
    }

    registry := guard.DefaultRegistry(&cfg.Guard)
    hctx := &guard.HookContext{
        Input:     input,
        Config:    &cfg.Guard,
        SessionID: input.SessionID,
        CWD:       input.CWD,
        Agent:     cfg.Agent, // (Finding 1) Pass agent from config to HookContext
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

        // Block short-circuits immediately.
        // stderr format preserved: "edi-guard: <reason>" (Finding 3)
        if result.Block {
            fmt.Fprintf(os.Stderr, "edi-guard: %s\n", result.Reason)
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

// loadGuardConfig, mergeGuardOverlay, loadYAMLInto, guardConfigFile,
// guardConfigOverlay remain in main.go — unchanged from current code.
// (Finding 5: config loading is binary-specific and stays in main.go)
```

**Finding 3 resolution:** The stderr format for block messages is preserved as
`edi-guard: <reason>` — matching the current output. The policy `Name()` is
**not** included in stderr output. Claude Code and users may parse stderr
messages; changing the format is a breaking behavioral change we avoid.
The `Name()` method exists for structured logging, debugging, and test
assertions — not for user-facing output.

**Finding 5 resolution:** Config loading stays in `main.go` as
`loadGuardConfig()`. There is no `guard.LoadConfig()`. The dispatcher code
calls `loadGuardConfig(input.CWD)` (local to `main` package), then passes
the resolved `config.GuardConfig` and agent string to `guard.DefaultRegistry()`
and `guard.HookContext`.

**Finding 12 resolution:** The import block includes `"encoding/json"`.

### 6. Shared Types and Helpers

#### `edi/internal/guard/types.go` — Exported types

```go
package guard

import "encoding/json"

// HookInput represents the JSON Claude Code sends to command hooks on stdin.
type HookInput struct {
    SessionID     string          `json:"session_id"`
    CWD           string          `json:"cwd"`
    HookEventName string          `json:"hook_event_name"`
    ToolName      string          `json:"tool_name"`
    ToolInput     json.RawMessage `json:"tool_input"`
    Error         string          `json:"error"`
    Trigger       string          `json:"trigger"`
}

// BashToolInput is the tool_input shape for Bash tool calls.
type BashToolInput struct {
    Command string `json:"command"`
}
```

#### `edi/internal/guard/response.go` — Exported functions

```go
package guard

import (
    "encoding/json"
    "io"
    "os"
    "strings"
)

// ParseStdin reads and decodes the hook input from stdin.
func ParseStdin() *HookInput {
    data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
    if err != nil {
        return nil
    }
    var input HookInput
    if err := json.Unmarshal(data, &input); err != nil {
        return nil
    }
    if input.CWD == "" {
        input.CWD, _ = os.Getwd()
    }
    return &input
}

// ParseBashInput extracts the command from Bash tool_input JSON.
// Exported: called by both the main.go dispatcher and FailureLoopPolicy.
// (Finding 6)
func ParseBashInput(raw json.RawMessage) *BashToolInput {
    var b BashToolInput
    if err := json.Unmarshal(raw, &b); err != nil {
        return nil
    }
    return &b
}

// BuildPreToolUseResponse constructs the JSON response for PreToolUse.
// advisories is a slice; multiple advisories are joined with newlines
// to produce the single additionalContext string required by the
// Claude Code hook protocol. (Finding 4)
func BuildPreToolUseResponse(command string, advisories []string, modified bool) map[string]interface{} {
    hso := map[string]interface{}{
        "hookEventName":      "PreToolUse",
        "permissionDecision": "allow",
    }
    if modified {
        hso["updatedInput"] = map[string]string{
            "command": command,
        }
    }
    if len(advisories) > 0 {
        hso["additionalContext"] = strings.Join(advisories, "\n")
    }
    return map[string]interface{}{
        "hookSpecificOutput": hso,
    }
}
```

**Finding 4 resolution:** `BuildPreToolUseResponse` accepts `[]string` for
advisories. The joining logic is explicit: `strings.Join(advisories, "\n")`.
The Claude Code hook protocol's `additionalContext` field is a single string,
so all advisories are concatenated with newline separators.

#### `edi/internal/guard/state.go` — Unexported shared helpers

```go
package guard

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// guardState is the file-based state for the failure loop counter.
// Unexported: internal to the guard package. (Finding 8)
type guardState struct {
    ConsecutiveFailures int    `json:"consecutive_failures"`
    Advised             bool   `json:"advised"`
    LastFailureCommand  string `json:"last_failure_command"`
    LastFailureError    string `json:"last_failure_error"`
}

// stateFilePath, readState, writeState are unexported package-internal
// helpers shared by FailureLoopPolicy and CompactionSnapshotPolicy.
// (Finding 8)

func stateFilePath(sessionID string) string {
    return filepath.Join(os.TempDir(), fmt.Sprintf("edi-guard-%s.json", sessionID))
}

func readState(sessionID string) guardState {
    if sessionID == "" {
        return guardState{}
    }
    data, err := os.ReadFile(stateFilePath(sessionID))
    if err != nil {
        return guardState{}
    }
    var s guardState
    if err := json.Unmarshal(data, &s); err != nil {
        return guardState{}
    }
    return s
}

func writeState(sessionID string, state guardState) {
    if sessionID == "" {
        return
    }
    data, err := json.Marshal(state)
    if err != nil {
        return
    }
    _ = os.WriteFile(stateFilePath(sessionID), data, 0600)
}
```

**Finding 8 resolution:** `guardState`, `readState()`, `writeState()`, and
`stateFilePath()` are all **unexported** in `guard/state.go`. They are
package-internal shared utilities used by `FailureLoopPolicy` and
`CompactionSnapshotPolicy`, not part of the public API.

### 7. Package Layout

```
edi/internal/guard/
├── policy.go                  # Interfaces: PreToolUsePolicy, PostToolUsePolicy, etc.
│                              # HookContext (with Agent field), PolicyResult
├── registry.go                # Registry type + validated Register() + accessor methods
├── defaults.go                # DefaultRegistry() constructor
├── types.go                   # HookInput, BashToolInput (exported)
├── state.go                   # guardState, readState, writeState (unexported)
├── response.go                # ParseStdin, ParseBashInput (exported), BuildPreToolUseResponse
├── policy_denylist.go         # DenyListPolicy + compiledPattern
├── policy_denylist_test.go    # Deny list tests (migrated from main_test.go)
├── policy_buildtag.go         # BuildTagPolicy + injectBuildTags, hasAllTags, etc.
├── policy_buildtag_test.go    # Build tag + hasAllTags tests
├── policy_failureloop.go      # FailureLoopPolicy
├── policy_failureloop_test.go # Failure counter tests
├── policy_compaction.go       # CompactionSnapshotPolicy + readActiveTasks, gitBranch, taskInfo
├── policy_compaction_test.go  # PreCompact tests
├── registry_test.go           # Registry ordering, short-circuit, merging, validation tests
└── dispatch_test.go           # Integration: full PreToolUse dispatch scenarios

edi/cmd/edi-guard/
├── main.go                    # Thin dispatcher (~90 lines) + config loading (~60 lines)
└── main_test.go               # Config loading tests (remain here — test main package)
```

### 8. Response Merging Rules

The dispatcher merges `PolicyResult` values from multiple policies. The rules:

| Field | Merge Strategy | Rationale |
|-------|---------------|-----------|
| `Block` | First block wins, short-circuits | Deny-list stops everything |
| `ModifiedCommand` | Chain — each policy receives previous policy's output | Build tags -> future "normalize paths" policy |
| `Advisory` | Collect into `[]string`, join with `\n` in `BuildPreToolUseResponse` | Multiple policies may have different warnings |

This means `EvalPreToolUse` receives the **current** command string (possibly
already modified by an earlier policy), not the original. This is essential for
policy chaining — a future "path normalization" policy could clean up a command
before the build-tag policy inspects it.

### 9. What Adding a New Policy Looks Like

Example: a "warn on untested changes" policy that detects `git commit` when
tests haven't been run in the current session.

```go
// edi/internal/guard/policy_commitguard.go
package guard

import (
    "context"
    "strings"
)

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

### 10. What Stays in `main.go`

- `main()` function with stdin parsing, `.edi/` check, config loading
- The four `dispatch*` functions (~60 lines total)
- Config loading: `loadGuardConfig()`, `mergeGuardOverlay()`, `loadYAMLInto()`,
  `guardConfigFile`, `guardConfigOverlay` — binary-specific YAML merge logic.
  **These stay in `main.go`, not in the `guard` package.** (Finding 5)

### 11. Migration Strategy

This is a refactor, not a rewrite. The migration preserves exact behavior:

**Step 1: Create `edi/internal/guard/` with interfaces and registry.**
No behavior change. Just new files.

**Step 2: Extract each policy into its own file.**
Move code from `main.go` into `guard/policy_*.go`. Each extraction is a pure
code move — the logic doesn't change.

**Step 3: Move tests alongside policies.**
Split `main_test.go` into `policy_*_test.go` files. Tests call the same
functions but through the policy interface. See §Testing for structural
changes required.

**Step 4: Rewrite `main.go` as thin dispatcher.**
Replace the four `handle*` functions with the `dispatch*` functions that loop
over the registry.

**Step 5: Add `registry_test.go` with integration tests.**
Test ordering, short-circuit, response merging, and `Register()` validation
as new behaviors.

Each step is a separate commit. At every step, `go test ./edi/cmd/edi-guard/...`
and `go test ./edi/internal/guard/...` must pass.

## Behavioral Notes

### stderr format (Finding 3)

The stderr format for block messages is **intentionally preserved** as:

```
edi-guard: <reason>
```

The policy `Name()` is **not** included in user-facing stderr output. This
avoids breaking any downstream parsing by Claude Code or user scripts. The
`Name()` method serves debugging, logging, and test assertions only.

If a verbose/debug mode is added in the future, the format could be:

```
edi-guard [deny-list]: <reason>
```

But this is opt-in, not the default.

## Testing

### Preserved tests (migrated with structural changes)

All 42 existing tests in `main_test.go` move to the appropriate
`policy_*_test.go` files. **Assertions are preserved, but call sites change**
to use the policy types instead of free functions. (Finding 10)

#### Build tag and hasAllTags tests — minimal changes

Tests for `injectBuildTags`, `hasAllTags`, etc. call package-internal
(unexported) functions. Since the test files are in `package guard`, they
can call these directly. Only the package declaration changes:

```go
// Before (main_test.go):
package main

func TestInjectBuildTags_Missing(t *testing.T) {
    modified, result := injectBuildTags("go test ./...", []string{"fts5"})
    ...
}

// After (policy_buildtag_test.go):
package guard

func TestInjectBuildTags_Missing(t *testing.T) {
    modified, result := injectBuildTags("go test ./...", []string{"fts5"})
    ...
}
```

#### Deny list tests — structural change required

Deny list tests currently use a test helper that calls the unexported
`compileDenyPatterns()` directly. After the refactor, pattern compilation is
internal to `DenyListPolicy`. Tests construct a policy and call
`EvalPreToolUse()`: (Finding 10)

```go
// Before (main_test.go):
var defaultPatterns = compileDenyPatterns(config.DefaultConfig().Guard.DenyPatterns)

func TestDenyList_ForceMain(t *testing.T) {
    reason := checkDenyList("git push --force origin main", defaultPatterns)
    if reason == "" {
        t.Fatal("expected deny")
    }
}

// After (policy_denylist_test.go):
var defaultPolicy = NewDenyListPolicy(config.DefaultConfig().Guard.DenyPatterns)

func TestDenyList_ForceMain(t *testing.T) {
    result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push --force origin main")
    if result == nil || !result.Block {
        t.Fatal("expected deny")
    }
}

func TestDenyList_SafeCommand(t *testing.T) {
    result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "go test -tags fts5 ./...")
    if result != nil {
        t.Fatalf("expected allow, got block: %q", result.Reason)
    }
}
```

The assertion logic (what's blocked, what's allowed) is identical. The call
site changes from `checkDenyList() -> reason string` to
`EvalPreToolUse() -> *PolicyResult`.

#### compileDenyPatterns test — moves into constructor test

```go
// Before:
func TestCompileDenyPatterns_InvalidPattern(t *testing.T) {
    compiled := compileDenyPatterns(patterns)
    if len(compiled) != 1 { ... }
}

// After:
func TestNewDenyListPolicy_InvalidPattern(t *testing.T) {
    policy := NewDenyListPolicy(patterns)
    // Valid pattern blocks, invalid is skipped
    result := policy.EvalPreToolUse(context.Background(), nil, "valid match")
    if result == nil || !result.Block { ... }
    // Verify the invalid pattern was silently skipped by testing
    // that only the valid pattern produces blocks
}
```

#### Failure counter tests — minimal changes

State helper tests (`readState`/`writeState`) remain unchanged since the
helpers are unexported within the same package.

#### PreCompact tests — structural change required

Tests currently pass `*hookInput` and `*guardConfigFile`. After the refactor,
they construct `*HookContext` with the `Agent` field:

```go
// Before:
cfg := &guardConfigFile{
    Guard: config.GuardConfig{BuildTags: []string{"fts5"}},
    Agent: "coder",
}
handlePreCompact(input, cfg)

// After:
policy := NewCompactionSnapshotPolicy(&config.GuardConfig{BuildTags: []string{"fts5"}})
hctx := &HookContext{
    Input:     input,
    Config:    &config.GuardConfig{BuildTags: []string{"fts5"}},
    SessionID: "test-compact",
    CWD:       dir,
    Agent:     "coder",
}
policy.OnPreCompact(context.Background(), hctx)
```

#### Response building tests — signature change

`buildPreToolUseResponse` becomes `BuildPreToolUseResponse` with `[]string`
advisories:

```go
// Before:
resp := buildPreToolUseResponse("go test -tags fts5 ./...", "advisory", true)

// After:
resp := BuildPreToolUseResponse("go test -tags fts5 ./...", []string{"advisory"}, true)

// Advisory-only (no modification):
resp := BuildPreToolUseResponse("", []string{"you're stuck"}, false)

// Both:
resp := BuildPreToolUseResponse("go test -tags fts5 ./...", []string{"you're stuck"}, true)
```

### New tests

| Test | File | Purpose |
|------|------|---------|
| `TestRegistry_OrderPreserved` | `registry_test.go` | Policies evaluate in registration order |
| `TestRegistry_BlockShortCircuits` | `registry_test.go` | After a Block, later policies don't run |
| `TestRegistry_CommandChaining` | `registry_test.go` | Policy B sees Policy A's modified command |
| `TestRegistry_AdvisoriesMerge` | `registry_test.go` | Two advisories are joined with newline |
| `TestRegistry_EmptyResultSkipped` | `registry_test.go` | Nil result from policy is ignored |
| `TestRegistry_RegisterInvalidPanics` | `registry_test.go` | `Register(42)` panics (Finding 2) |
| `TestDispatch_PreToolUse_BashOnly` | `dispatch_test.go` | Non-Bash tools are ignored |
| `TestDispatch_PostToolUse_AllFire` | `dispatch_test.go` | All PostToolUse policies run |
| `TestDefaultRegistry_HasAllPolicies` | `defaults_test.go` | DefaultRegistry returns 4 policies |

### Test migration mapping

| Current test | Moves to | Change type |
|---|---|---|
| `TestInjectBuildTags_*` (10 tests) | `policy_buildtag_test.go` | Package declaration only |
| `TestHasAllTags_*` (4 tests) | `policy_buildtag_test.go` | Package declaration only |
| `TestDenyList_*` (12 tests) | `policy_denylist_test.go` | Structural: free function -> policy method |
| `TestCompileDenyPatterns_*` (1 test) | `policy_denylist_test.go` | Structural: test via constructor + eval |
| `TestFailureCounter_*` (2 tests) | `policy_failureloop_test.go` | Package declaration only |
| `TestPreCompact_*` (3 tests) | `policy_compaction_test.go` | Structural: HookContext with Agent field |
| `TestBuildPreToolUseResponse_*` (3 tests) | `response_test.go` | Structural: string -> []string advisory |
| `TestLoadGuardConfig_*` (7 tests) | stay in `main_test.go` | No change |

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
`CompactionSnapshotPolicy`. They move to `guard/state.go` as unexported
package-level functions, available to all policies within the package.
(Finding 8)

### 5. Registry accessor allocation (Finding 13)

Registry accessor methods (`PreToolUsePolicies()`, etc.) allocate a new slice
on each call by iterating the full policy list and type-asserting each entry.
For 4 policies this is negligible (~64 bytes). If the policy count grows past
20 and profiling shows this matters, cache the filtered slices at construction
time in `Register()`. Until then, the simple approach is correct.

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

5. **No non-Bash tool guarding.** All PreToolUse/PostToolUse/PostToolUseFailure
   dispatchers filter on `ToolName == "Bash"`. This matches the hook
   registration in `.claude/settings.json`. Adding non-Bash policies requires
   both dispatcher and hook registration changes. (Finding 7)

6. **No panic recovery in policies.** A panic in any policy crashes the hook
   process. This is a programmer error, not a runtime condition. Claude Code
   continues if the hook fails. (Finding 11)

## Implementation Order

| Step | Description |
|------|-------------|
| 1 | Create `guard/` package: interfaces, registry (with validation), types, response, state |
| 2 | Extract `DenyListPolicy` + migrate tests (structural: free function -> policy method) |
| 3 | Extract `BuildTagPolicy` + migrate tests (package declaration change only) |
| 4 | Extract `FailureLoopPolicy` + migrate tests (package declaration change only) |
| 5 | Extract `CompactionSnapshotPolicy` + helpers + migrate tests (structural: HookContext with Agent) |
| 6 | Write `DefaultRegistry` + `defaults_test.go` |
| 7 | Rewrite `main.go` as thin dispatcher (config loading stays) |
| 8 | Add registry integration tests (including Register validation panic test) |
| 9 | Verify all 42 existing tests pass with migrated call sites |

---

## Appendix: Findings Resolution Summary

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | **Critical** | CompactionSnapshotPolicy can't access `Agent` | Added `Agent string` to `HookContext`. `main.go` sets it from `guardConfigFile.Agent`. |
| 2 | **Critical** | `Register()` accepts any type | Added runtime validation — panics if value implements no policy interface. |
| 3 | **Behavioral** | stderr format changes | Preserved existing `edi-guard: <reason>` format. `Name()` not in stderr. |
| 4 | **Underspecified** | Advisory joining logic | Explicit: `strings.Join(advisories, "\n")` in `BuildPreToolUseResponse`. |
| 5 | **Contradiction** | Config loading location | Config loading stays in `main.go`. No `guard.LoadConfig()`. |
| 6 | **Underspecified** | `ParseBashInput` export status | **Exported** in `guard/response.go`. Called by both dispatcher and policies. |
| 7 | **Design gap** | Bash-only filtering | Documented as intentional constraint in design constraints §6 and §What This Does NOT Do. |
| 8 | **Underspecified** | `guardState` export status | All unexported in `guard/state.go`: `guardState`, `readState()`, `writeState()`, `stateFilePath()`. |
| 9 | **Underspecified** | CompactionSnapshot helper locations | `readActiveTasks()`, `gitBranch()`, `taskInfo` move to `policy_compaction.go` as unexported helpers. |
| 10 | **Inaccurate** | "Tests pass without modification" | Corrected: assertions preserved, call sites change. Structural changes for deny-list, compaction, and response tests. Detailed before/after shown. |
| 11 | **Missing** | No panic recovery stance | Stated as deliberate non-goal in design constraints §7 and §What This Does NOT Do §6. |
| 12 | **Bug** | Missing `encoding/json` import | Added to dispatcher import block. |
| 13 | **Minor** | Registry slice allocation | Documented as acceptable trade-off in registry code comment and Risks §5. |
