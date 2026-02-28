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
    pre_commit_checks: true
    destructive_guard: true
    write_audit: false       # opt-in, adds latency
    telemetry: true
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

## Recommendation

Start with **Phase 1** (settings generation) — it's low-effort plumbing that unlocks all webhook hook functionality through EDI's config system. Follow immediately with a minimal **Phase 2** sidecar implementing the destructive command guard, as it provides immediate safety value with low risk of disrupting developer flow.

The pre-commit quality gate (opportunity #1) is the highest-value integration for AEF's mission, but it requires flight recorder query capability in the sidecar, making it a Phase 2b item.

Convention enforcement via PostToolUse (opportunity #3) is attractive but carries latency risk and should be opt-in. Telemetry (opportunity #4) is low-risk and high-value — implement early.

## Open Questions

1. **Process lifecycle**: Since EDI uses `syscall.Exec`, the sidecar must be a separate process. Should it be managed by EDI (started before exec) or by a systemd/launchd service?
2. **Settings file ownership**: `.claude/settings.json` may have user customizations beyond hooks. EDI should merge, not overwrite. Same challenge as `.mcp.json` — the `UpdateMCPConfig` pattern applies.
3. **Agent-specific policies**: Should hook behavior change when switching modes via `/plan`, `/build`, `/review`? The webhook endpoint has access to `EDI_AGENT_MODE` via headers.
4. **Migration path**: Should `task-sync-hook` (currently a command hook) migrate to webhook format for consistency, or keep both patterns?
