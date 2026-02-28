# Webhook Hooks Exploration: External Validation for AEF

**Date**: 2026-02-28
**Status**: Exploration / RFC
**Author**: Claude (EDI session)

## Context

Claude Code now supports **webhook hooks** (HTTP hooks) — a mechanism where tool-use events are POSTed to external HTTP endpoints that can validate, block, modify, or audit actions in real time. This is a significant expansion beyond the existing shell command hooks that AEF already uses (e.g., `task-sync-hook` on `SessionStart`).

This document explores how webhook hooks align with AEF's core mission of producing reliable, high-quality codegen output, and identifies concrete integration opportunities.

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
| **13** | PreCompact preservation | Context loss is real, but a webhook can't know what's "critical" without understanding the conversation. Improve what goes into `/memories/` and CLAUDE.md instead. |
| **6** | Task completion validation | "Are tests written? Build passing?" — the sidecar can't answer these without running the tests itself. Checking for test file existence is a poor proxy. |
| **12** | Agent-mode enforcement | Assumes modes are strict access-control boundaries. In practice, reviewers make quick fixes, architects prototype. Blocking writes in reviewer mode forces tedious mode-switching for legitimate workflows. |
| **11** | Refactoring safety gate | Requires Go AST analysis in the sidecar. Heavy engineering for infrequent scenarios with brittle detection (is adding a parameter a "signature change" or "extending an API"?). |
| **10** | Scope creep detection | "High file count + diverse packages" isn't an algorithm, it's a judgment call. Any threshold you pick will have high false positives, training users to ignore the warnings — which is worse than not having them. |

## Architecture: EDI Webhook Service

### Option A: Sidecar Process (Recommended for v0)

EDI launches a lightweight HTTP server alongside Claude Code, rather than replacing itself entirely.

```
edi launch
  ├── Start webhook service on localhost:9090
  ├── Write .claude/settings.json with hook config
  ├── Write .mcp.json with RECALL config (existing)
  └── syscall.Exec → Claude Code
```

**Pros**: Simple, no external dependencies, shares local context (files, RECALL DB).
**Cons**: Must manage process lifecycle (or use a goroutine before exec). The `syscall.Exec` model means EDI replaces itself — the sidecar would need to be a separate binary.

**Implementation approach**: Build `edi-hooks` as a separate binary (like `task-sync-hook`) that EDI starts in the background before exec'ing Claude Code. Configure it via EDI config.

```yaml
# .edi/config.yaml
hooks:
  enabled: true
  port: 9090
  build_tags: ["fts5"]              # Required build tags for go test/build
  destructive_deny_list:            # Commands to block (regex patterns)
    - "rm -rf .edi"
    - "git push --force.*main"
    - "git push --force.*master"
  failure_loop_threshold: 3         # Inject warning after N consecutive failures
```

### Option B: External Service (Team/Enterprise)

For teams, the webhook endpoint is a shared service:

```
hooks:
  enabled: true
  url: https://edi-hooks.internal.company.com
  auth_token_env: EDI_HOOK_TOKEN
```

**Pros**: Centralized policy enforcement, team-wide audit trail.
**Cons**: Network latency, requires infrastructure, auth management.

### Option C: Hybrid

Local sidecar for low-latency validation (destructive guards, convention checks), external service for audit logging and team policies.

## Implementation Sketch

### Phase 1: Settings Generation (Low effort, high value)

Extend `edi launch` to write `.claude/settings.json` with webhook hook configuration, pointing at whatever endpoints are configured. This is purely plumbing — it makes webhook hooks configurable through EDI's config system.

**Changes**:
- Add `HooksConfig` to `config/schema.go`
- Add `WriteHooksSettings()` to `launch/` package
- Call from `runLaunch()` alongside `UpdateMCPConfig()`

### Phase 2: Local Validation Sidecar (Medium effort, high value)

Build `edi-hooks` binary that serves the validation endpoints.

**Priority endpoints**:
1. `POST /hooks/pre-tool-use` — Destructive command guard
2. `POST /hooks/post-tool-use` — Telemetry logging
3. `POST /hooks/task-completed` — Completion validation

**Changes**:
- New `edi/cmd/edi-hooks/` binary
- Policy engine in `edi/internal/hooks/policies/`
- Telemetry sink in `edi/internal/hooks/telemetry/`
- Update Makefile to build and install

### Phase 3: RECALL Integration (Medium effort, medium value)

Connect the hooks service to the RECALL database for enrichment and feedback.

- PreToolUse enrichment queries RECALL
- PostToolUseFailure feeds failure patterns back to RECALL
- Session telemetry informs RECALL relevance scoring

### Phase 4: Team Policies (Higher effort, enterprise value)

Shared webhook service with:
- Configurable policy rules (YAML/JSON)
- Team-wide audit dashboard
- Integration with CI/CD and PR review

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Latency impact on developer flow | Local sidecar (< 10ms). Timeout configuration. Non-blocking for observational hooks. |
| Over-blocking frustrates users | Start with deny-list only (known-dangerous). Convention checks are advisory (PostToolUse context injection, not blocking). |
| Sidecar lifecycle management | EDI starts it, Claude Code's session doesn't depend on it (non-2xx = proceed). Crash = graceful degradation. |
| Configuration complexity | Sensible defaults in EDI config. `hooks.enabled: true` with preset policies. |
| Security of local endpoint | Localhost only. Token-based auth for remote endpoints. |

## Relationship to Existing AEF Components

| Component | Relationship to Webhook Hooks |
|-----------|------------------------------|
| **Skills** (coding, testing) | Hooks *enforce* what skills *suggest*. Skills guide Claude's reasoning; hooks validate the output. |
| **Flight recorder** | Hooks feed additional telemetry into the same audit trail. |
| **RECALL** | Hooks can query RECALL for enrichment and feed observations back. |
| **task-sync-hook** | Existing command hook pattern. Webhook hooks are the HTTP evolution. Could migrate `task-sync-hook` to webhook format for consistency. |
| **Agents** | Agent-specific hook policies (e.g., reviewer mode has stricter write guards). |
| **`/end` workflow** | SessionEnd hook can automate parts of the capture workflow. |

## Revised Implementation Plan

### What to build

1. **Settings generation plumbing** — Extend `edi launch` to write `.claude/settings.json` with hook configuration, same merge pattern as `UpdateMCPConfig`. This is pure plumbing that unlocks any future hooks.

2. **Minimal sidecar (`edi-hooks`)** with exactly two policies:
   - **Build tag enforcement (#9)** — `PreToolUse` on Bash: if command matches `go (test|build)` and doesn't include `-tags.*fts5`, deny with corrected command. String match, 10 lines of logic.
   - **Destructive command guard (#2)** — `PreToolUse` on Bash: deny-list of `rm -rf .edi`, `git push --force.*main`, `DROP TABLE`, etc. Another 20 lines of logic.

3. **Failure loop breaker (#14)** — Add a simple counter in the sidecar: if the same tool + command pattern fails 3+ times, inject "Repeated failure — consider a different approach." This is a counter, not a pattern engine.

### What to defer until there's evidence

- **Pre-commit quality gate (#1)** — Measure how often Claude actually skips quality checks first. If it's rare, the simpler fix is baking `go vet && go test -tags fts5 -race ./... &&` into the commit step of the skill.
- **Session telemetry (#4)** — Build the consumer (dashboard, eval integration) first. Then add the data source.

### What not to build

Everything in Tiers 3 and 4. These are either duplicating existing tools (go vet, staticcheck, RECALL MCP), solving problems at the wrong layer (compaction handling belongs in prompts, not webhooks), or attempting algorithmic judgments (scope creep, completion validation) that produce more false positives than signal.

## Recommendation

The honest scope of genuinely valuable webhook hooks for AEF is small: **two blocking guards and a failure counter**. That's roughly 50-80 lines of policy logic in a sidecar, plus the plumbing to configure it.

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

## Open Questions

1. **Process lifecycle**: Since EDI uses `syscall.Exec`, the sidecar must be a separate process. EDI starts it in the background before exec'ing Claude Code. What's the cleanest way to ensure it shuts down when the Claude Code session ends? PID file + signal? Automatic exit on stdin close?
2. **Settings file ownership**: `.claude/settings.json` may have user customizations beyond hooks. EDI should merge, not overwrite — same pattern as `UpdateMCPConfig` for `.mcp.json`.
3. **Migration path**: Should `task-sync-hook` (currently a command hook) migrate to webhook format, or keep both patterns? The command hook is simpler for its purpose — no reason to change it unless maintaining both becomes burdensome.
4. **Command hook alternative**: For the three policies identified, would command hooks (which AEF already uses for `task-sync-hook`) be simpler than webhook hooks? Command hooks avoid the HTTP server lifecycle entirely. The trade-off is that webhook hooks are more natural for a future team/shared service, while command hooks are simpler for local-only use.
