# Hooks Exploration: External Validation for AEF

**Date**: 2026-02-28 (updated 2026-03-01)
**Status**: Exploration / RFC
**Author**: Claude (EDI session)

## Context

Claude Code supports four hook types — command, HTTP webhook, prompt, and agent — that can validate, block, modify, or audit actions in real time. AEF already uses command hooks (`task-sync-hook` on `SessionStart`).

This document explores how hooks align with AEF's quality mission, identifies 14 potential opportunities, honestly assesses which are valuable vs. performative complexity, and recommends a minimal implementation using command hooks.

## How Webhook Hooks Work

### Lifecycle

```
Claude Code event fires (e.g., PreToolUse for a Bash command)
  → Matcher regex checked (e.g., "Bash")
  → HTTP POST sent to configured endpoint with event JSON
  → Endpoint returns 2xx with JSON decision
  → Claude Code applies decision: allow / deny / modify
```

### Key Properties

| Property | Webhook Hooks | Command Hooks (current) |
|----------|--------------|------------------------|
| Transport | HTTP POST/response | stdin/stdout |
| Blocking | 2xx + JSON `permissionDecision: "deny"` | Exit code 2 |
| Modification | Response body injected as context | stdout injected |
| Auth | HTTP headers with env var interpolation | N/A |
| Failure mode | Non-2xx = non-blocking (proceeds) | Non-zero exit = non-blocking |
| Shared infra | Centralized service for teams | Per-machine binary |
| Latency | Network round-trip | Local process |

### Event Types Relevant to AEF

| Event | Fires When | AEF Opportunity |
|-------|-----------|-----------------|
| `PreToolUse` | Before any tool executes | **Validate** commands, file writes, git operations |
| `PostToolUse` | After tool succeeds | **Audit** what was done, **analyze** output quality |
| `PostToolUseFailure` | After tool fails | **Log** failures for pattern detection |
| `Stop` | Claude finishes responding | **Verify** response completeness |
| `PreCompact` | Before context compaction | **Preserve** critical context |
| `SessionStart` | Session begins | **Enrich** with project policies (already done via command hook) |
| `SessionEnd` | Session terminates | **Capture** session metrics |
| `TaskCompleted` | Task marked done | **Validate** task completion criteria |

### Configuration Format

Configured in `.claude/settings.json` (project-level) or `~/.claude/settings.json` (global):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "http",
            "url": "http://localhost:9090/hooks/pre-tool-use",
            "timeout": 10,
            "headers": {
              "Authorization": "Bearer $EDI_HOOK_TOKEN",
              "X-Session-ID": "$EDI_SESSION_ID"
            },
            "allowedEnvVars": ["EDI_HOOK_TOKEN", "EDI_SESSION_ID"]
          }
        ]
      }
    ]
  }
}
```

## Alignment with AEF's Mission

AEF exists to produce **reliable, high-quality codegen output** through continuity (RECALL), structured workflows (agents/skills), and curated knowledge. Webhook hooks open up a new enforcement layer that operates *below* the prompt level — at the infrastructure level where actions are taken.

### Current Quality Model

```
Prompt-level enforcement (skills, agents, system prompt)
  ↓
Claude's judgment (applies skills, follows instructions)
  ↓
Tool execution (no validation — trust the model)
  ↓
Human review (post-hoc)
```

### With Webhook Hooks

```
Prompt-level enforcement (skills, agents, system prompt)
  ↓
Claude's judgment (applies skills, follows instructions)
  ↓
Pre-execution validation (webhook hook — ENFORCED)     ← NEW
  ↓
Tool execution
  ↓
Post-execution analysis (webhook hook — OBSERVATIONAL)  ← NEW
  ↓
Human review (informed by hook telemetry)
```

The key insight: **webhook hooks move quality enforcement from "suggested" (prompt-level) to "enforced" (infrastructure-level)** for the policies that matter most.

## Concrete Integration Opportunities

### 1. Pre-Commit Quality Gate (PreToolUse → Bash)

**Problem**: AEF's coding and testing skills instruct Claude to run `go vet`, `staticcheck`, and tests before committing. But this is prompt-level guidance — it can be skipped under pressure or context loss.

**Solution**: A `PreToolUse` webhook that intercepts `git commit` commands and validates that quality checks have been run in the current session.

```
PreToolUse (matcher: "Bash", command contains "git commit")
  → POST to EDI validation service
  → Service checks flight_recorder_log for recent:
      - go vet pass
      - go test pass
      - staticcheck pass
  → If missing: deny with reason "Run quality checks first: go vet, go test, staticcheck"
  → If present: allow
```

**Impact**: Hard enforcement of the "pre-flight checklist" from the testing skill.

### 2. Destructive Command Guard (PreToolUse → Bash)

**Problem**: Claude Code already has some built-in protections, but project-specific destructive patterns (e.g., `DROP TABLE`, `rm -rf .edi/`, force-pushing to main) aren't covered.

**Solution**: A `PreToolUse` webhook that evaluates Bash commands against a project-specific deny-list with contextual rules.

```
PreToolUse (matcher: "Bash")
  → POST command to validation service
  → Service evaluates against rules:
      - Block: rm -rf on project root or .edi/
      - Block: git push --force to main/master
      - Block: DROP/TRUNCATE on production-tagged databases
      - Warn: commands touching .claude/ or .mcp.json directly
  → deny/allow with explanation
```

**Impact**: Safety net that catches mistakes without slowing down normal development.

### 3. Write Audit & Convention Enforcement (PostToolUse → Write/Edit)

**Problem**: AEF's coding skill defines conventions (error handling, naming, etc.) but enforcement relies entirely on Claude following the prompt.

**Solution**: A `PostToolUse` webhook that analyzes file changes for convention violations.

```
PostToolUse (matcher: "Edit|Write")
  → POST file path + content to analysis service
  → Service runs lightweight checks:
      - Go files: gofumpt compliance, error handling patterns
      - New files: check if they follow project structure conventions
      - Test files: verify table-driven test pattern usage
  → Return observations as context injected into conversation
```

**Impact**: Real-time feedback loop. Not blocking (PostToolUse can't undo), but the injected context prompts Claude to fix issues immediately.

### 4. Session Telemetry & Flight Recorder Enhancement (Multiple Events)

**Problem**: The flight recorder captures what Claude *chooses* to log. It misses tool-level granularity — how many files were edited, what commands failed, time between actions.

**Solution**: Webhook hooks on multiple events feeding a telemetry service:

```
PostToolUse (all tools)     → Log: tool name, duration, success
PostToolUseFailure (all)    → Log: failure type, command, error
Stop                        → Log: response metrics
SessionEnd                  → Log: session summary, total tool calls
```

**Impact**: Builds a comprehensive session audit trail. Enables:
- Detecting sessions that went off-track (high failure rate)
- Measuring actual quality check compliance across sessions
- Identifying patterns in tool failures (input for RECALL)

### 5. RECALL-Enriched Pre-Execution Context (PreToolUse → Bash)

**Problem**: When Claude runs a build or deploy command, it may not have checked RECALL for known issues with that operation.

**Solution**: A `PreToolUse` webhook that queries RECALL for relevant warnings.

```
PreToolUse (matcher: "Bash", command contains "make|go build|deploy")
  → POST command to enrichment service
  → Service queries RECALL for items tagged with the command/context
  → Returns relevant warnings as context:
      "RECALL note: 'go build' requires -tags fts5 for SQLite support (captured 2026-02-15)"
  → Claude sees this context and adjusts
```

**Impact**: Proactive knowledge surfacing at the moment of action, not just at session start.

### 6. Task Completion Validation (TaskCompleted)

**Problem**: Tasks can be marked complete without verification that acceptance criteria were met.

**Solution**: A `TaskCompleted` webhook that validates completion against the task manifest.

```
TaskCompleted
  → POST task ID + session context to validation service
  → Service loads task from .edi/tasks/active.yaml
  → Checks: tests written? files changed match scope? build passing?
  → If insufficient: inject context "Task may not be complete — missing test coverage for X"
```

**Impact**: Lightweight guardrail against premature task completion.

## Additional Quality-Focused Opportunities

The following opportunities emerge from a deep analysis of every AEF skill, agent, and subagent — identifying prompt-level rules that would benefit most from infrastructure-level enforcement.

### 7. Test Integrity Enforcement (PostToolUse → Edit)

**Problem**: The testing skill declares three **FORBIDDEN** actions and one **REQUIRED** action. These are the hardest rules in the AEF skill set, yet they are enforced only via prompt.

```
❌ FORBIDDEN: Changing assertions to match broken behavior
❌ FORBIDDEN: Deleting tests that "don't work anymore"
❌ FORBIDDEN: Adding t.Skip() without justification
✅ REQUIRED: Escalate unexpected failures for evaluation
```

**Solution**: A `PostToolUse` webhook on Edit/Write that inspects changes to `*_test.go` files.

```
PostToolUse (matcher: "Edit|Write", file matches "*_test.go")
  → Parse diff to detect:
      1. Test function deletion (func Test* removed)   → WARN "Test deleted — escalate if intentional"
      2. t.Skip() addition without preceding comment    → WARN "t.Skip() requires justification"
      3. Assertion value change without code change      → WARN "Assertion changed to match broken behavior?"
  → Also detect anti-patterns from testing skill:
      4. os.Chdir() in test file                        → WARN "Use path arguments instead"
      5. os.MkdirTemp without cleanup                   → WARN "Use t.TempDir()"
      6. Helper function missing t.Helper()             → WARN "Add t.Helper() for correct line numbers"
  → Inject findings as context — Claude fixes immediately
```

**Impact**: The FORBIDDEN rules become observable violations that Claude must acknowledge and address, rather than silent omissions. This is the single highest-ROI hook for test quality.

### 8. Error Handling Pattern Detection (PostToolUse → Edit)

**Problem**: The coding skill has strong rules about error handling — never discard errors, always wrap with context, use sentinel errors. These are the most commonly violated Go conventions.

**Solution**: A `PostToolUse` webhook on Edit/Write for `*.go` files that scans for error anti-patterns.

```
PostToolUse (matcher: "Edit|Write", file matches "*.go" but not "*_test.go")
  → Scan changed lines for:
      1. _, _ :=  or  _ =  discarding errors     → WARN "Error discarded — handle or log explicitly"
      2. return err  without fmt.Errorf wrapping  → SUGGEST "Wrap with context: fmt.Errorf('x: %w', err)"
      3. panic() for error handling                → WARN "Return error instead of panic"
      4. init() with side effects                  → WARN "Use explicit initialization"
  → Non-blocking: inject as advisory context
```

**Impact**: Catches the most common Go quality issues at write time. Advisory, not blocking — Claude sees the feedback and self-corrects.

### 9. Build Tag Enforcement (PreToolUse → Bash)

**Problem**: AEF requires `-tags "fts5"` for all Go test and build commands due to SQLite FTS5 dependency. This is documented in CLAUDE.md but easy to forget after context compaction.

**Solution**: A `PreToolUse` webhook that intercepts `go test` and `go build` commands.

```
PreToolUse (matcher: "Bash", command matches "go (test|build)")
  → Check if command includes -tags "fts5" or -tags fts5
  → If missing AND project uses SQLite (detected from go.mod):
      DENY with reason: "This project requires -tags fts5. Use: go test -tags fts5 ./..."
```

**Impact**: Eliminates a recurring failure mode. Extremely low latency (string match), zero false positives.

### 10. Scope Creep Detection (PostToolUse → Edit, PreToolUse → Bash)

**Problem**: The coding skill's "Change Scope Discipline" section warns against mixing bug fixes with refactoring. This is prompt-level guidance that erodes under pressure.

**Solution**: A two-part webhook that tracks change scope within a session.

```
PostToolUse (matcher: "Edit|Write")
  → Track: files modified, functions changed, lines added/removed
  → Maintain session-local change profile:
      - File count
      - Whether changes span multiple packages
      - Whether function signatures changed (refactoring signal)

PreToolUse (matcher: "Bash", command contains "git commit")
  → Analyze accumulated change profile:
      - If high file count + diverse packages → SUGGEST "Consider splitting into focused commits"
      - If signature changes + behavior changes → WARN "Mixing refactoring with behavior changes"
  → Non-blocking: inject as advisory context
```

**Impact**: Promotes the "one change type per commit" discipline from the coding skill.

### 11. Refactoring Safety Gate (PreToolUse → Edit)

**Problem**: The refactoring-planning and scaffolding-tests skills require creating scaffolding tests *before* modifying function signatures. This multi-step workflow is easy to skip.

**Solution**: A `PreToolUse` webhook that detects refactoring intent.

```
PreToolUse (matcher: "Edit", file matches "*.go" but not "*_test.go")
  → Parse the edit to detect function/method signature changes:
      - Return type changed
      - Parameters added/removed/reordered
      - Function renamed
  → If signature change detected:
      Check for matching scaffolding test (TestScaffold_* in same package)
      → If missing: WARN "Before modifying signatures, create scaffolding tests.
                          See: scaffolding-tests skill"
  → Allow (advisory, not blocking) — but the injected context triggers Claude's
    scaffolding-tests skill awareness
```

**Impact**: Bridges the gap between the refactoring-planning skill (which describes the process) and actual enforcement. Even as a non-blocking warning, it's highly effective because it triggers Claude's existing skill knowledge.

### 12. Agent-Mode Policy Enforcement (PreToolUse → Edit/Write)

**Problem**: The reviewer agent should analyze code, not modify it. The architect agent should design, not implement. But mode boundaries are prompt-level only — nothing prevents a reviewer from writing code.

**Solution**: A `PreToolUse` webhook that enforces mode-appropriate behavior.

```
PreToolUse (matcher: "Edit|Write")
  → Check X-Agent-Mode header (set from EDI_AGENT_MODE env var)
  → If mode == "reviewer":
      WARN "You are in reviewer mode. Suggest changes rather than making them.
            Switch to /build mode to implement."
  → If mode == "architect":
      WARN "You are in architect mode. Focus on design.
            Switch to /build mode to implement."
```

**Impact**: Reinforces the agent/mode boundaries that are central to AEF's workflow design. Prevents the common pattern of a review session drifting into implementation.

### 13. PreCompact Context Preservation (PreCompact)

**Problem**: When Claude Code compacts the conversation to fit context limits, critical session state (active task context, RECALL findings, review verdicts) can be lost.

**Solution**: A `PreCompact` webhook that ensures critical state is persisted before compaction.

```
PreCompact
  → POST session state to preservation service
  → Service writes critical context to /memories/session-cache.md:
      - Current task ID and status
      - Recent RECALL search results marked as "kept"
      - Active plan review verdict
      - Accumulated change profile (from opportunity #10)
  → Return context summary for injection into post-compaction state
```

**Impact**: Directly addresses the context loss problem that undermines quality after compaction. The pre-commit quality gate (opportunity #1) is most at risk here — without this, compaction could erase the memory that quality checks were (or weren't) run.

### 14. Failure Pattern Detection & Auto-RECALL (PostToolUseFailure)

**Problem**: When tools fail repeatedly, it usually indicates a systemic issue (wrong build flags, missing dependency, environment misconfiguration). These patterns are valuable RECALL candidates but are typically lost.

**Solution**: A `PostToolUseFailure` webhook that detects repeated failure patterns.

```
PostToolUseFailure (all tools)
  → Log failure: tool, command, error message, timestamp
  → Detect patterns:
      - Same command failing 3+ times → INJECT "Repeated failure detected.
            Consider a different approach or check environment setup."
      - Same error across different commands → INJECT "Systemic issue:
            [error pattern]. Check [likely root cause]."
  → After session: surface failure patterns as RECALL capture candidates
    during /end workflow
```

**Impact**: Catches the "stuck in a loop" failure mode and promotes it to actionable knowledge. Feeds directly into the `/end` session capture workflow.

## Honest Assessment: Value vs. Performative Complexity

Not all 14 opportunities are created equal. Most are either duplicating existing tooling, solving problems that prompting already handles, or adding infrastructure complexity disproportionate to the failure rate they address.

### Tier 1: Genuinely Valuable (build these)

| # | Opportunity | Why it's real |
|---|------------|---------------|
| **9** | Build tag enforcement | Concrete, recurring failure. Claude forgets `-tags fts5` after compaction. String match, zero false positives, trivial to implement. This is a project-specific `.editorconfig` equivalent. |
| **2** | Destructive command guard | Low-frequency, catastrophic-consequence. Deny-list is cheap insurance. The value is entirely in the 1-in-100 session where it prevents `rm -rf .edi/` or force-push to main. |
| **14** | Failure pattern detection | Claude tends to retry failing commands with small variations rather than stepping back. An external "you've been stuck for 5 rounds" signal genuinely changes behavior. But only as a simple counter — not a "pattern detection engine." |
| **13** | PreCompact state snapshot | Compaction causes real session degradation — Claude loses track of task context, build requirements, and prior decisions. A command hook that mechanically writes 5-10 lines of facts to `/memories/compaction-state.md` before compaction preserves these across the boundary. Not a "what's critical?" judgment engine — just: task ID, branch, build tags, last test result, active plan. |

### Tier 2: Conditionally Valuable (build only if the precondition is met)

| # | Opportunity | Precondition |
|---|------------|--------------|
| **4** | Session telemetry | Only valuable if something consumes the data — a dashboard, eval pipeline, or analysis tool. Without a reader, it's write-only bytes. Build the consumer first. |
| **1** | Pre-commit quality gate | Only valuable if Claude actually skips quality checks frequently enough to justify the state-tracking infrastructure. Measure the failure rate first. A simpler fix may be to bake `go vet && go test` into the commit command in the skill itself. |

### Tier 3: Sounds Good, Actually Duplicative (don't build)

| # | Opportunity | What it's really doing |
|---|------------|----------------------|
| **8** | Error handling detection | Reimplementing `go vet` and `staticcheck` inside a webhook. Those tools already exist, are more comprehensive, and are already in the pre-commit checklist. |
| **3** | Write audit & conventions | Same as #8 but broader. You're building a linter in a webhook. Use the actual linters. |
| **7** | Test integrity enforcement | Claude violates the FORBIDDEN rules in two cases: user asks it to (webhook won't override), or context loss (real fix is better compaction, not a PostToolUse regex). And "assertion changed to match broken behavior" is semantically impossible to detect — you'd need to understand intent, not syntax. |

### Tier 4: Solving the Wrong Problem at the Wrong Layer (don't build)

| # | Opportunity | Why it's wrong-layered |
|---|------------|----------------------|
| **5** | RECALL enrichment | The sidecar would need the full codex library, embedding models, and DB access. RECALL is already an MCP tool Claude can call proactively. The fix is a better prompt, not duplicating the MCP server in a webhook. |
| **13** | PreCompact preservation | ~~Originally Tier 4~~ → **Moved to Tier 1** after confirming compaction causes real degradation. The key: don't try to be smart about what's critical. Just persist mechanical facts (task ID, branch, build tags, last test result). |
| **6** | Task completion validation | "Are tests written? Build passing?" — the sidecar can't answer these without running the tests itself. Checking for test file existence is a poor proxy. |
| **12** | Agent-mode enforcement | Assumes modes are strict access-control boundaries. In practice, reviewers make quick fixes, architects prototype. Blocking writes in reviewer mode forces tedious mode-switching for legitimate workflows. |
| **11** | Refactoring safety gate | Requires Go AST analysis in the sidecar. Heavy engineering for infrequent scenarios with brittle detection (is adding a parameter a "signature change" or "extending an API"?). |
| **10** | Scope creep detection | "High file count + diverse packages" isn't an algorithm, it's a judgment call. Any threshold you pick will have high false positives, training users to ignore the warnings — which is worse than not having them. |

## Architecture: Choosing the Right Hook Type

Claude Code supports four hook types. For AEF's three Tier 1 policies, only one makes sense.

### Hook Type Comparison for AEF's Policies

| | Command Hook | HTTP Webhook | Prompt Hook | Agent Hook |
|---|---|---|---|---|
| **Mechanism** | Binary on stdin/stdout | POST to localhost | Single-turn LLM call | Multi-turn subagent |
| **Latency** | ~5ms (process spawn) | ~10ms (HTTP round-trip) | ~500ms-3s (API call) | ~5-30s (agentic loop) |
| **Can block** | Exit code 2 | JSON `permissionDecision: "deny"` | `{"ok": false}` | `{"ok": false}` |
| **Maintains state** | File-based (hacky but works) | In-memory (clean) | No | No |
| **Needs server process** | No | Yes (lifecycle management) | No | No |
| **Existing AEF pattern** | Yes (`task-sync-hook`) | No | No | No |
| **Cost per invocation** | Zero | Zero | API tokens | API tokens |

### Why command hooks win for these policies

**Build tag enforcement (#9)** is checking whether a string contains `-tags fts5`. A regex. Using an LLM for this (prompt/agent hook) is like using a neural network to check if a number is even — slower, more expensive, less reliable, and harder to debug than a 5ms process that runs `strings.Contains()`.

**Destructive command guard (#2)** is matching against a deny-list of regex patterns. Same argument.

**Failure loop breaker (#14)** needs a counter across invocations. A command hook can write `{"go test": 3}` to `/tmp/edi-guard-{session}.json` and read it on the next invocation. Slightly inelegant, but it's ~10 lines of code vs. an entire HTTP server lifecycle.

An HTTP webhook sidecar's only advantage over command hooks is in-memory state for the counter. That's not worth introducing process lifecycle management (who starts it? who kills it when the session ends? what if it crashes?).

Prompt/agent hooks are designed for genuinely complex validation — "run the test suite and verify it passes before letting Claude stop." That's a real use case for an LLM with tool access. String matching is not.

### Recommendation: Single Command Hook Binary (`edi-guard`)

Build one Go binary that handles multiple hook events, same pattern as `task-sync-hook`. The binary reads `hook_event_name` from the input JSON to determine which logic path to run:

```
PreToolUse fires → edi-guard → check deny-list, build tags, failure counter
PreCompact fires → edi-guard → snapshot task ID, branch, build tags to /memories/
PostToolUseFailure fires → edi-guard → increment failure counter
```

```go
// edi-guard: ~200 lines of Go
// 1. Parse hook JSON from stdin (includes hook_event_name, session_id, cwd)
// 2. Switch on hook_event_name:
//
//    "PreToolUse" + tool_name == "Bash":
//      a. Check command against deny-list patterns → exit 2 if match
//      b. Check if command matches "go (test|build)" without "-tags fts5" → exit 2 if missing
//      c. Check failure counter → if threshold exceeded, inject advisory on stdout
//
//    "PostToolUseFailure":
//      a. Increment failure counter in /tmp/edi-guard-{session}.json
//
//    "PreCompact":
//      a. Read current task from .edi/tasks/active.yaml
//      b. Read git branch
//      c. Write /memories/compaction-state.md with mechanical facts:
//         - Active task ID + description
//         - Current git branch
//         - Required build tags (from config)
//         - Last test/build pass or fail (from failure counter state)
//      d. Exit 0 (PreCompact hooks can't block)
```

EDI's `launch.go` writes the hook config to `.claude/settings.json` using the same merge pattern as `UpdateMCPConfig`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": ".*",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": ".*",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ]
  }
}
```

```yaml
# .edi/config.yaml
guard:
  enabled: true
  build_tags: ["fts5"]
  deny_patterns:
    - "rm -rf .edi"
    - "git push --force.*main"
    - "git push --force.*master"
  failure_loop_threshold: 3
```

**What this gives you:**
- Same proven pattern as `task-sync-hook` (already works)
- No HTTP server, no port, no process lifecycle
- ~5ms overhead per Bash command (imperceptible)
- ~100-150 lines of Go, buildable in an afternoon
- Configuration via EDI's existing YAML system
- Graceful degradation (if `edi-guard` crashes, Claude Code proceeds)

### When to reconsider HTTP webhooks or agent hooks

- **HTTP webhooks**: If AEF gains a team mode where multiple developers share policies via a centralized service. That's an enterprise feature that doesn't exist yet — don't build for it.
- **Prompt hooks**: If a future policy requires semantic judgment (not regex). Example: "Is this SQL query potentially destructive?" can't be regex-matched because `DELETE FROM users WHERE id = 5` is fine but `DELETE FROM users` isn't, and the distinction is semantic.
- **Agent hooks**: If a policy needs to read files or run commands to make a decision. Example: "Before allowing Claude to stop, verify the test suite passes." That needs tool access.

## Relationship to Existing AEF Components

| Component | Relationship |
|-----------|-------------|
| **Skills** (coding, testing) | `edi-guard` enforces what skills suggest. Skills guide reasoning; the guard catches mechanical failures (wrong flags, dangerous commands). |
| **task-sync-hook** | Proven pattern. `edi-guard` follows the same architecture: Go binary, JSON on stdin, exit codes. |
| **RECALL** | No direct relationship. RECALL's quality problems are internal (see "Not the Right Layer" section). |
| **Agents** | No agent-specific hook policies needed. Mode enforcement via hooks is performative (see Tier 4 assessment). |

## Revised Implementation Plan

### What to build

1. **Settings generation plumbing** — Extend `edi launch` to write `.claude/settings.json` with hook configuration, using the same merge pattern as `UpdateMCPConfig` for `.mcp.json`. Add `GuardConfig` to `config/schema.go`.

   Changes:
   - `edi/internal/config/schema.go` — Add `GuardConfig` struct
   - `edi/internal/launch/settings.go` — New file: `UpdateHooksSettings()` (mirrors `mcp.go` pattern)
   - `edi/internal/cli/launch.go` — Call `UpdateHooksSettings()` from `runLaunch()`

2. **`edi-guard` command hook binary** — Single binary handling three events, same pattern as `task-sync-hook`:

   **PreToolUse (Bash):**
   - **Build tag enforcement (#9)** — If command matches `go (test|build)` without `-tags.*fts5`, exit 2 with corrected command.
   - **Destructive command guard (#2)** — If command matches deny-list patterns, exit 2 with explanation.
   - **Failure loop breaker (#14)** — Read counter from `/tmp/edi-guard-{session}.json`. If threshold exceeded, inject advisory on stdout.

   **PostToolUseFailure (all tools):**
   - Increment failure counter in `/tmp/edi-guard-{session}.json`.

   **PreCompact:**
   - **Compaction state snapshot (#13)** — Write `/memories/compaction-state.md` with mechanical facts: active task ID + description, git branch, required build tags, last test result.

   Changes:
   - `edi/cmd/edi-guard/main.go` — New binary (~200 lines)
   - `edi/Makefile` — Build and install alongside `task-sync-hook`

### What to defer until there's evidence

- **Pre-commit quality gate (#1)** — Measure how often Claude actually skips quality checks first. If it's rare, the simpler fix is baking `go vet && go test -tags fts5 -race ./... &&` into the commit step of the skill.
- **Session telemetry (#4)** — Build the consumer (dashboard, eval integration) first. Then add the data source.

### What not to build

Everything in Tiers 3 and 4. These are either duplicating existing tools (go vet, staticcheck, RECALL MCP), solving problems at the wrong layer (compaction handling belongs in prompts, not webhooks), or attempting algorithmic judgments (scope creep, completion validation) that produce more false positives than signal.

## Recommendation

The honest scope of genuinely valuable hooks for AEF is small: **two blocking guards, a failure counter, and a compaction snapshot**. That's ~200 lines of Go in a single binary, plus the plumbing to configure it.

This is not a failure of the webhook hooks feature — it's a reflection that AEF's prompt-level enforcement (skills, agents, RECALL) already works well for most quality concerns. Hooks add value specifically where:

1. The failure is **mechanical** (wrong flags, dangerous commands), not **judgmental** (code quality, scope management)
2. The check is **trivially decidable** (string match, counter), not **semantically complex** (intent detection, coverage analysis)
3. Prompt-level enforcement **actually fails** in practice, not just in theory

Resist the temptation to build a sophisticated policy engine. The 14-policy config with phased rollout looks impressive in a design doc, but most of those policies either duplicate existing tooling or attempt to algorithmically enforce what is fundamentally a judgment call.

## Webhook Hooks for RECALL: Not the Right Layer

An attractive idea is using webhook hooks to improve RECALL retrieval quality — observing search results, tracking what Claude does with them, and feeding signals back. After analysis, this doesn't hold up.

**RECALL's actual quality problems are internal to the search engine:**
- The `feedback` table captures user signals but the search engine never reads them when ranking
- Flight recorder `retrieval_judgment` entries log kept/dropped decisions but nothing feeds this back to scoring
- RRF produces narrow score distributions that the retrieval-judge skill itself calls unreliable
- Reranking is optional, untested, and likely not running in most deployments

**Why webhooks can't fix these:** Webhooks observe tool calls from the outside. They can see that `recall_search` was called and what it returned, but they can't change how the engine ranks results. The signal that RECALL needs (feedback → better ranking) must flow inside the search engine, not through an external HTTP observer.

**The plausible webhook ideas and why they're not worth it:**
- *Implicit relevance* (correlate retrieved items with subsequent file edits): Fragile inference, requires cross-event state tracking, uncertain signal quality
- *Query quality monitoring* (warn on vague queries): The retrieval-judge skill already does this via prompting
- *Auto-capture failures*: Bypasses AEF's "prompted capture" philosophy and would pollute the knowledge base with unvetted items
- *Usage counting*: Belongs inside the MCP server as a one-line counter, not in a webhook sidecar

**What would actually improve RECALL:**
1. Make the search engine read the `feedback` table and adjust ranking scores
2. Convert flight recorder `retrieval_judgment` entries into feedback signals automatically
3. Ship and enable reranking models by default
4. Add test coverage for chunking, embeddings, and MCP protocol

These are codex-internal improvements, not webhook problems.

## Other Claude Code Features Assessed

These features were evaluated for AEF relevance and found to be either premature or irrelevant:

| Feature | Verdict | Why |
|---------|---------|-----|
| **Worktree isolation for subagents** | Not yet | AEF's 7 subagents are pre-configured agent definitions with no parallel orchestration code. Parallelism is documented in the spec but delegated to Claude Code's native Tasks mechanism, which AEF doesn't invoke programmatically. Worktree isolation matters the day you build an orchestration layer — which is a separate, larger effort. |
| **MCP Tool Search (dynamic loading)** | Irrelevant | RECALL exposes 5 tools (~1500 tokens of schema). That's 0.75% of a 200K context window. Dynamic loading triggers at 10%. You'd need ~65 tools before this matters. |
| **PreCompact hook** | **Build it** | Compaction causes confirmed session degradation. Moved to Tier 1. Implemented as part of `edi-guard`: writes mechanical facts (task ID, branch, build tags, last test result) to `/memories/compaction-state.md` before compaction. Not a judgment engine — just a fact dump. |
| **Memory leak fixes** | Already shipped | Tree-sitter WASM, MCP cache leaks, task output retention — all fixed in recent Claude Code releases. Update Claude Code if you haven't. No AEF work needed. |

## Open Questions

1. **Settings file ownership**: `.claude/settings.json` may have user customizations beyond hooks. EDI should merge, not overwrite — same read-merge-write pattern as `UpdateMCPConfig` for `.mcp.json`.
2. **Config-driven deny-list**: Should deny patterns live in `.edi/config.yaml` (project-specific, checked into repo) or in `~/.edi/config.yaml` (global, personal)? Project-level makes sense for team conventions; global makes sense for personal safety rails. Probably both, merged at load time.
3. **Compaction state content**: The `/memories/compaction-state.md` file needs to be kept small (it persists across compaction and consumes context). What's the right set of mechanical facts? Candidate list: task ID + one-line description, git branch, required build tags, last test pass/fail, current agent mode. Anything more risks the "trying to be smart about what's critical" trap.
