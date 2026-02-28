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

## Opportunity Priority Matrix

| # | Opportunity | Event | Enforcement | Latency | Value | Effort |
|---|------------|-------|-------------|---------|-------|--------|
| 1 | Pre-commit quality gate | PreToolUse | Blocking | Low | **Critical** | Medium |
| 2 | Destructive command guard | PreToolUse | Blocking | Low | **Critical** | Low |
| 9 | Build tag enforcement | PreToolUse | Blocking | Trivial | **High** | Low |
| 7 | Test integrity enforcement | PostToolUse | Advisory | Low | **High** | Medium |
| 8 | Error handling detection | PostToolUse | Advisory | Low | **High** | Medium |
| 4 | Session telemetry | Multiple | Observational | None | **High** | Low |
| 14 | Failure pattern detection | PostToolUseFailure | Advisory | Low | **High** | Medium |
| 13 | PreCompact preservation | PreCompact | Preservational | Low | **High** | Medium |
| 12 | Agent-mode enforcement | PreToolUse | Advisory | Trivial | **Medium** | Low |
| 10 | Scope creep detection | Multiple | Advisory | Low | **Medium** | Medium |
| 5 | RECALL enrichment | PreToolUse | Advisory | Medium | **Medium** | Medium |
| 6 | Task completion validation | TaskCompleted | Advisory | Low | **Medium** | Medium |
| 11 | Refactoring safety gate | PreToolUse | Advisory | Low | **Medium** | High |
| 3 | Write audit & conventions | PostToolUse | Advisory | Medium | **Medium** | High |

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
  policies:
    # Phase 1: Blocking guards (low latency, high confidence)
    build_tag_enforcement: true    # #9  — Require -tags fts5 for go test/build
    destructive_guard: true        # #2  — Block rm -rf .edi/, force-push main
    agent_mode_enforcement: true   # #12 — Warn on mode-inappropriate actions

    # Phase 2: Observational (advisory context injection)
    telemetry: true                # #4  — Log all tool events
    test_integrity: true           # #7  — Detect test deletion, t.Skip abuse
    error_handling: true           # #8  — Detect discarded errors, bare returns
    failure_patterns: true         # #14 — Detect repeated failures

    # Phase 2b: Stateful enforcement
    pre_commit_checks: true        # #1  — Require vet/test/staticcheck before commit
    compaction_safety: true        # #13 — Preserve critical context before compaction

    # Phase 3: Advanced (opt-in, higher latency)
    recall_enrichment: false       # #5  — Query RECALL before build/deploy
    scope_creep: false             # #10 — Track change diversity within session
    refactoring_safety: false      # #11 — Require scaffolding tests for sig changes
    task_validation: false         # #6  — Validate task completion criteria
    write_audit: false             # #3  — Full convention analysis on writes
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

## Revised Implementation Phases

### Phase 1: Settings Generation + Trivial Guards (1-2 days)

Extend `edi launch` to write `.claude/settings.json` with webhook hook configuration. Implement the simplest blocking hooks in the sidecar:

- **Build tag enforcement (#9)** — Pure string match, zero false positives, immediate value
- **Destructive command guard (#2)** — Deny-list only, well-scoped
- **Agent-mode enforcement (#12)** — Header check, trivial logic

These three hooks require minimal infrastructure and provide immediate safety value.

### Phase 2: Observational Hooks (3-5 days)

Add the non-blocking hooks that observe and advise:

- **Session telemetry (#4)** — Log all tool events to a local SQLite DB
- **Test integrity enforcement (#7)** — PostToolUse analysis of test file changes
- **Error handling detection (#8)** — PostToolUse analysis of Go file changes
- **Failure pattern detection (#14)** — PostToolUseFailure pattern matching

These are advisory — they inject context but never block. Low risk, high signal.

### Phase 2b: Pre-Commit Gate + Compaction Safety

- **Pre-commit quality gate (#1)** — Requires telemetry from Phase 2 to know whether checks were run
- **PreCompact preservation (#13)** — Requires session state tracking from Phase 2

### Phase 3: RECALL Integration + Advanced Policies

- **RECALL enrichment (#5)** — Query RECALL DB from the sidecar
- **Scope creep detection (#10)** — Requires multi-tool state tracking
- **Refactoring safety gate (#11)** — Requires Go AST analysis
- **Task completion validation (#6)** — Requires manifest access

### Phase 4: Team Policies

Shared webhook service, configurable rule engine, audit dashboard.

## Recommendation

Start with **Phase 1** — it's 1-2 days of work that delivers three blocking guards with near-zero false positive risk. Build tag enforcement (#9) alone will eliminate a recurring failure mode that costs minutes per occurrence.

Follow with **Phase 2** observational hooks. The test integrity enforcement (#7) is the single highest-ROI webhook for AEF's quality mission: the testing skill's FORBIDDEN rules are the hardest quality boundaries in the system, and moving them from prompt-level to infrastructure-level is a step change in enforcement confidence.

Phase 2b's pre-commit gate (#1) is the crown jewel — it closes the loop between "skills say run checks" and "checks were actually run" — but it depends on the telemetry infrastructure from Phase 2.

## Open Questions

1. **Process lifecycle**: Since EDI uses `syscall.Exec`, the sidecar must be a separate process. Should it be managed by EDI (started before exec) or by a systemd/launchd service?
2. **Settings file ownership**: `.claude/settings.json` may have user customizations beyond hooks. EDI should merge, not overwrite. Same challenge as `.mcp.json` — the `UpdateMCPConfig` pattern applies.
3. **Agent-specific policies**: Should hook behavior change when switching modes via `/plan`, `/build`, `/review`? The webhook endpoint has access to `EDI_AGENT_MODE` via headers. Opportunity #12 provides basic enforcement, but deeper mode-aware policies (e.g., reviewer can run tests but not edit source) would require richer configuration.
4. **Migration path**: Should `task-sync-hook` (currently a command hook) migrate to webhook format for consistency, or keep both patterns?
5. **Go AST analysis**: Opportunities #8 (error handling), #10 (scope creep), and #11 (refactoring safety) benefit from Go AST parsing rather than regex. Should the sidecar include `go/parser` for structural analysis, or keep checks regex-based for simplicity?
6. **Telemetry storage**: Session telemetry (#4) needs a local store. Reuse the RECALL SQLite database (simpler) or a separate `~/.edi/telemetry.db` (cleaner separation)?
7. **Hook composition**: If multiple hooks fire on the same event (e.g., build tag enforcement AND RECALL enrichment on `go test`), how should their responses compose? First-deny-wins? Merge all advisory context?
