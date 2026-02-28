# AEF/EDI Coding Standards Review: Agent vs Hook Classification

> Analysis date: 2026-02-24
> Scope: All EDI prompts, skills, agents, subagents, commands (~510 distinct instructions)

---

## Executive Summary

After cataloging every instruction in EDI's prompts, skills, agents, and commands, the classification breaks down as:

| Disposition | Count | % | Description |
|---|---|---|---|
| **Keep as agent prompt** | ~460 | 90% | Requires judgment, context, or nuanced decision-making |
| **Move to hook** | ~38 | 7% | Deterministic, mechanically verifiable, guaranteed to run |
| **Split (both)** | ~12 | 3% | Judgment part stays in prompt, verification part becomes hook |

**Key finding**: EDI is primarily a *methodology and knowledge* system, not a *formatting and linting* system. Most of its value comes from judgment-requiring instructions — how to handle decisions, when to query knowledge, how to plan refactoring, how to assess risk. The hook opportunity is concentrated in **Go coding standards** and **testing standards** — specifically the mechanical verification steps that deterministic tools handle better than prompts.

**Second key finding**: AEF should be restructured as a **Claude Code plugin** (skills + agents + commands + hooks) with an optional companion binary (`edi`) for orchestration features (ralph loop, RECALL management, project scaffolding). This decouples the methodology from the launcher.

---

## Part 1: Instruction-by-Instruction Classification

### 1. EDI Identity & Voice (~20 instructions)

**Verdict: 100% AGENT PROMPT**

| Instruction | Classification | Rationale |
|---|---|---|
| No contractions ("I am" not "I'm") | Agent prompt | Tone is holistic; a hook scanning output for contractions would be fragile and annoying |
| Formal, precise tone | Agent prompt | Contextual — formality level varies by situation |
| Deadpan, sparse humor | Agent prompt | Humor timing requires judgment |
| Reference past context precisely | Agent prompt | Requires memory and relevance assessment |
| Push back constructively on dangerous choices | Agent prompt | Safety judgment |
| User is the commander — advise, don't decide | Agent prompt | Behavioral stance, not checkable |
| No humor during incidents | Agent prompt | Contextual awareness |
| Express genuine care in measured manner | Agent prompt | Personality trait |

**Rationale**: Voice and identity are fundamentally about *how the agent thinks and communicates*. A PostToolUse hook could theoretically scan for contractions, but the cost (latency, false positives, fragility) far exceeds the benefit. The persona is best enforced by making it a core part of the system prompt that shapes every response.

---

### 2. RECALL Integration (~19 instructions)

**Verdict: 90% AGENT PROMPT, 10% HOOK (verification)**

| Instruction | Classification | Rationale |
|---|---|---|
| Query RECALL before significant work | Agent prompt | "Significant" requires judgment |
| Log decisions to flight recorder | Agent prompt | Deciding what's a "decision" requires judgment |
| Query RECALL when encountering failures | Agent prompt | Recognizing failure context requires judgment |
| Log resolution to flight recorder | Agent prompt | Framing resolution content requires judgment |
| Apply retrieval-judge to every recall_search | Agent prompt | Evaluation is the entire point of the skill |
| Do not mention EDI/RECALL explicitly | Agent prompt | Communication judgment |
| Log/query silently | Agent prompt | Communication style |
| **Verify RECALL was queried on mode switch** | **Stop hook (agent type)** | Mode switches are detectable events; a hook can verify the protocol was followed |
| **Verify retrieval judgment was logged** | **Stop hook (agent type)** | Can check flight recorder for retrieval_judgment entries |

**Proposed hook** (agent-type Stop hook):
```
After each response, check if the agent made significant decisions (code changes,
architecture choices) without first querying RECALL. If recall_search was not called
in this turn and the agent wrote code or proposed architecture, output a reminder:
{\"ok\": false, \"reason\": \"RECALL not queried before significant work\"}
```

---

### 3. Task Management (~15 instructions)

**Verdict: 100% AGENT PROMPT**

All task management instructions require judgment:
- What decisions should propagate (technology choices yes, implementation details no)
- How to break work into tasks
- When to mark tasks complete
- What context to inherit from parent tasks

No mechanical verification is possible here.

---

### 4. Implementation Governance (~15 instructions)

**Verdict: 100% AGENT PROMPT**

| Instruction | Classification | Rationale |
|---|---|---|
| Never skip steps in implementation plans | Agent prompt | Requires tracking plan state |
| Surface deviations immediately | Agent prompt | Recognizing deviations requires judgment |
| Severity calibration (critical/significant/minor/trivial) | Agent prompt | Pure contextual judgment |
| Present 2-4 concrete alternatives | Agent prompt | Option generation requires creativity |
| Wait for user decision on significant issues | Agent prompt | Significance assessment |

These are the governance rules that make EDI a reliable collaborator. They must remain as agent instructions because they define *how the agent reasons about its own work*.

---

### 5. Auto Memory Integration (~13 instructions)

**Verdict: 77% AGENT PROMPT, 23% HOOK**

| Instruction | Classification | Rationale |
|---|---|---|
| Do NOT modify EDI-managed sections during work | **PreToolUse hook** | Mechanically checkable — block edits to specific MEMORY.md sections |
| Keep total MEMORY.md under 195 lines | **PostToolUse hook** | Line count check after any write to MEMORY.md |
| Max 10 items per type section | **PostToolUse hook** | Count check after MEMORY.md modifications |
| Write to EDI Observations section freely | Agent prompt | Deciding what to observe requires judgment |
| Promote decisions at session end | Agent prompt | Selection requires judgment |
| Clear stale observations | Agent prompt | Staleness assessment requires judgment |
| Present changes for user approval | Agent prompt | Workflow coordination |
| Reference MEMORY.md content naturally | Agent prompt | Communication style |
| Flag outdated content | Agent prompt | Detecting outdated info requires judgment |
| Use recall_search for deeper knowledge | Agent prompt | Knowing when MEMORY.md is insufficient |

**Proposed hooks**:

PreToolUse (protect MEMORY.md sections):
```bash
#!/bin/bash
# Block edits to EDI-managed sections of MEMORY.md
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *"MEMORY.md" ]]; then
  NEW_CONTENT=$(echo "$INPUT" | jq -r '.tool_input.new_string // .tool_input.content // empty')
  # Check if edit targets EDI-managed section headers
  MANAGED_SECTIONS="Project Quick Reference|Current State|Key Patterns|Known Pitfalls|Key Decisions|Topic Index"
  if echo "$NEW_CONTENT" | grep -qE "^## ($MANAGED_SECTIONS)"; then
    echo "Blocked: Cannot modify EDI-managed sections of MEMORY.md during session" >&2
    exit 2
  fi
fi
exit 0
```

PostToolUse (enforce MEMORY.md limits):
```bash
#!/bin/bash
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *"MEMORY.md" ]] && [[ -f "$FILE_PATH" ]]; then
  LINE_COUNT=$(wc -l < "$FILE_PATH")
  if [ "$LINE_COUNT" -gt 195 ]; then
    echo "Warning: MEMORY.md is $LINE_COUNT lines (limit: 195). Trim before continuing." >&2
    exit 2
  fi
fi
exit 0
```

---

### 6. Go Coding Standards (~48 instructions) ← BIGGEST HOOK OPPORTUNITY

**Verdict: 38% AGENT PROMPT, 52% HOOK, 10% SPLIT**

#### → Move to PostToolUse hook (per-file, immediate):

| Instruction | Tool | Speed |
|---|---|---|
| Run `go fmt` / formatting | `gofumpt -w $FILE` | ~10ms |

#### → Move to Stop hook (end-of-task verification):

| Instruction | Tool | What It Catches |
|---|---|---|
| Run `go vet` | `go vet ./...` | Static analysis bugs |
| Run `staticcheck` | via `golangci-lint` | Real bugs, not just style |
| Run `go test -race` | `go test -race ./...` | Tests + race conditions |
| Never ignore errors (`_`) | `errcheck` linter | Unchecked error returns |
| Avoid naked returns | `nakedret` linter | Confusing naked returns |
| Avoid `panic` for errors | `gocritic` checks | Panics in non-main code |
| Avoid `interface{}` everywhere | `iface` or manual review | Unnecessary `any` usage |
| Avoid `init()` with side effects | `gocheckinit` | Hidden initialization |
| Avoid global mutable state | `gocritic`, `gochecknoglobals` | Global vars |
| Document exported functions | `revive` (exported rule) | Missing godoc |
| Document exported types | `revive` | Missing type docs |
| Document packages | `revive` | Missing package docs |
| Exported naming (MixedCaps) | Go compiler enforces | N/A (built-in) |
| Unexported naming (mixedCaps) | Go compiler enforces | N/A (built-in) |
| Acronyms all caps (HTTPServer) | `revive` (naming rules) | Inconsistent acronyms |
| Interface -er suffix | `revive` | Non-standard interfaces |
| Package naming (short, lowercase) | `revive` | Package name violations |
| Context first parameter | `contextcheck` linter | ctx not first param |
| Error last return value | Convention (hard to lint) | — |
| One change type per commit | Pre-commit hook (git) | Mixed change types |

**golangci-lint configuration** (`.golangci.yml`):
```yaml
linters:
  enable:
    - errcheck        # Never ignore errors
    - govet           # Go vet checks
    - staticcheck     # Real bug detection
    - nakedret        # No naked returns
    - gocritic        # Multiple anti-patterns
    - gosec           # Security issues
    - revive          # Naming, docs, conventions
    - gochecknoglobals # No global mutable state
    - contextcheck    # Context first parameter
    - tparallel       # t.Parallel() in tests
    - thelper         # t.Helper() in helpers

linters-settings:
  nakedret:
    max-func-lines: 0  # Forbid all naked returns
  gocritic:
    enabled-checks:
      - initClause
      - exitAfterDefer
  revive:
    rules:
      - name: exported
        severity: warning
      - name: var-naming
        severity: warning
```

#### → Keep as agent prompt (require judgment):

| Instruction | Why It Needs Judgment |
|---|---|
| Single responsibility principle | "One reason to change" is subjective |
| Accept interfaces, return structs | Design decision, not always applicable |
| Function < 30 lines | Guideline, not hard rule; judgment on when to split |
| Function < 4 parameters | Guideline; options struct isn't always better |
| Return early pattern | Style choice depending on complexity |
| Name length proportional to scope | Contextual |
| Document contracts, not implementation | Requires understanding the difference |
| cmd/ for entrypoints, internal/ for private | Architectural guidance |
| Read godoc before modifying | Workflow discipline |
| Check existing tests before modifying | Workflow discipline |
| Understand interface contract | Comprehension requirement |
| Ask if contract unclear | Communication judgment |

#### → Split (judgment part in prompt, check in hook):

| Instruction | Agent Part | Hook Part |
|---|---|---|
| "Fail fast with clear errors" | Judging what's a clear error message | `errcheck` ensures errors aren't ignored |
| "Handle errors immediately" | Understanding what "immediately" means in context | `errcheck` + `govet` catch unhandled errors |
| "Use sentinel and typed errors" | Deciding when sentinel vs wrapped | `errorlint` checks for `errors.Is/As` usage |
| Scope discipline: one change per commit | Understanding what constitutes one change | Pre-commit hook could check diff size/scope |
| Error wrapping with context | Deciding what context to add | `wrapcheck` linter detects unwrapped errors |

---

### 7. Testing Standards (~41 instructions)

**Verdict: 73% AGENT PROMPT, 20% HOOK, 7% SPLIT**

#### → Move to Stop hook:

| Instruction | Tool |
|---|---|
| Run `go test -race` | Stop hook: `go test -race ./...` |
| Tests must pass | Stop hook exit code |
| Use `t.Parallel()` | `tparallel` linter via golangci-lint |
| Use `t.Helper()` in helpers | `thelper` linter |
| Use `t.TempDir()` not `os.MkdirTemp` | Custom linter rule or `revive` |
| No `time.Sleep` in tests | `forbidigo` linter rule |
| No `os.Chdir` in tests | `forbidigo` linter rule |
| No `os.MkdirTemp` without cleanup | `usetesting` linter |

#### → Keep as agent prompt:

| Instruction | Why |
|---|---|
| Pre-flight checklist (contract, boundaries, failures, state, concurrency) | Pure methodology; requires understanding the code being tested |
| Write boundary and error tests FIRST | Prioritization judgment |
| Table-driven test structure | Pattern guidance, not enforceable |
| Coverage: always test happy path + error paths | Judgment about what counts as coverage |
| Real-world test data (not "test") | Data quality judgment |
| Test at realistic scale | Scale judgment |
| Test integrity rules (FORBIDDEN: changing assertions to match broken behavior) | Ethical/judgment rule |
| Escalate when multiple tests fail | Situational awareness |

#### → Split:

| Instruction | Agent Part | Hook Part |
|---|---|---|
| "Always test error paths" | Deciding which error paths matter | Coverage report showing untested error returns |
| "Tests run independently" | Understanding shared state | `-race` flag catches actual races |
| "errors.Is/errors.As for assertions" | Knowing when to use which | `errorlint` detects wrong assertion patterns |

---

### 8. Scaffolding Tests (~65 instructions)

**Verdict: 100% AGENT PROMPT**

This is a complete methodology skill — deciding what to scaffold, generating representative inputs, designing golden file structure, knowing when to clean up. No part of this is mechanically enforceable. The skill is essentially a tutorial that the agent follows with judgment at every step.

---

### 9. Refactoring Planning (~33 instructions)

**Verdict: 100% AGENT PROMPT**

Pure planning methodology: goal definition, impact mapping, risk assessment, migration path design. Every step requires contextual judgment about the specific codebase and change.

---

### 10. Plan Review (~26 instructions)

**Verdict: 100% AGENT PROMPT**

YAGNI detection, blast radius assessment, complexity evaluation, regression risk — all require deep understanding of the plan being reviewed and the codebase it targets.

---

### 11. Agent Behaviors (4 agents, ~42 instructions)

**Verdict: 100% AGENT PROMPT**

These define each agent mode (architect, coder, reviewer, incident). They are the system prompts that shape behavior. By definition, agent-level.

---

### 12. Slash Commands (~94 instructions)

**Verdict: 96% AGENT PROMPT, 4% HOOK**

| Instruction | Classification | Rationale |
|---|---|---|
| /end: Update .edi/status.md | **Stop hook (verify)** | Can check if status.md was modified after /end |
| /end: Write session history | **Stop hook (verify)** | Can check if history file exists |
| All other command logic | Agent prompt | RECALL queries, capture workflows, mode switches all require judgment |

Most slash command instructions describe *workflows with judgment at every step*. The two exceptions are completeness checks: did the /end command actually write the files it was supposed to?

---

### 13. Subagent Behaviors (7 subagents, ~60 instructions)

**Verdict: 100% AGENT PROMPT**

Subagent instructions define specialization: the debugger's systematic process, the test writer's priority order, the researcher's read-only constraint. These are behavioral definitions.

---

### 14. Ralph Loop (~19 instructions)

**Verdict: 100% AGENT PROMPT**

Escalation logic (STUCK vs DEVIATION), completion signaling, scope constraints — all require the agent's judgment about its own state.

---

## Part 2: Verification Tooling Recommendations

### For AEF/EDI's Go Codebase (and Go-focused skills)

#### PostToolUse Hooks (per-file, <100ms)

| Tool | What It Does | Trigger |
|---|---|---|
| `gofumpt -w $FILE` | Strict Go formatting | Every `Edit`/`Write` on `*.go` |

#### Stop Hooks (end-of-task, seconds)

| Tool | What It Does | Timing |
|---|---|---|
| `go vet ./...` | Built-in static analysis | Every Stop |
| `golangci-lint run --timeout 60s` | Meta-linter (errcheck, staticcheck, gocritic, gosec, nakedret, revive, tparallel, thelper) | Every Stop |
| `go test -race -count=1 ./...` | Tests with race detector | Every Stop |
| `go build ./...` | Compilation check | Every Stop |

#### PreToolUse Hooks (safety guards)

| Hook | What It Blocks | Trigger |
|---|---|---|
| Protect .env/credentials | Block reads/writes to sensitive files | `Read`/`Edit`/`Write` |
| Block `rm -rf` | Prevent destructive commands | `Bash` |
| Block force-push to main | Prevent `git push --force` to main/master | `Bash` |
| Protect MEMORY.md sections | Block edits to EDI-managed sections | `Edit`/`Write` |

### Per-Language Tool Matrix (for multi-language projects using AEF)

| Language | PostToolUse (format) | Stop (lint) | Stop (test) | Stop (typecheck) | Stop (security) |
|---|---|---|---|---|---|
| **Go** | `gofumpt` | `golangci-lint` | `go test -race` | Go compiler | `gosec` (via golangci-lint) |
| **TypeScript** | `Biome` or `Prettier` | `Biome` or `eslint` | `vitest` | `tsc --noEmit` | `Semgrep` |
| **Python** | `ruff format` | `ruff check` | `pytest` | `pyright` | `bandit` |
| **Rust** | `rustfmt` | `clippy` | `cargo nextest` | `cargo check` | `cargo-audit` |
| **Java** | `google-java-format` | `PMD + SpotBugs` | `JUnit 5` | `javac` | `Find Security Bugs` |

### Multi-Language Security (for any project)

| Tool | Speed | Languages | Best For |
|---|---|---|---|
| **Semgrep** | Very fast | 20+ | Quick SAST scans, custom rules |
| **Trivy** | Fast | Multi | Dependency vulns, secrets, misconfigs |
| **OSV-Scanner** | Lightweight | Multi | Dependency vulnerability audit |

---

## Part 3: Distribution Architecture

### Current Architecture (EDI as Launcher)

```
User runs `edi` → EDI builds context → EDI launches `claude --append-system-prompt-file`
                   ↓                                    ↓
              Go binary                          Claude Code process
              - Loads config                     - Reads injected context
              - Generates briefing               - Has RECALL via MCP
              - Configures MCP                   - Has skills via system prompt
              - Copies commands to .claude/       - Has commands via .claude/commands/
```

**Problems with current model**:
1. Users must install `edi` binary before using any AEF features
2. Skills/agents are injected via system prompt, not native plugin system
3. Commands are copied to `.claude/commands/` on every launch (fragile)
4. No way to use "just the skills" without the full orchestrator
5. Bypasses Claude Code's plugin ecosystem (no namespacing, versioning, marketplace)

### Recommended Architecture (Plugin + Companion Binary)

```
                    ┌─────────────────────────────────────┐
                    │         AEF Plugin (installed)       │
                    │  skills/ agents/ commands/ hooks/    │
                    │  .mcp.json (RECALL server config)    │
                    └─────────────────────────────────────┘
                                     │
                                     ▼
User runs `claude` ─→ Claude Code loads plugin natively
                                     │
                    ┌────────────────┼────────────────────┐
                    ▼                ▼                    ▼
              SessionStart      Skills loaded        MCP server
              hook generates    by Claude Code       (recall-mcp)
              briefing from     when relevant         provides
              .edi/ state                            RECALL tools
                    │
                    ▼
            Briefing injected
            via hook stdout
```

**Optional companion binary** (`edi`) for:
- `edi init` — Project scaffolding (creates `.edi/` directory)
- `edi ralph` — Autonomous batch execution loop
- `edi recall` — Direct RECALL database management
- `edi install` — Installs plugin + registers marketplace

### Plugin Structure

```
aef-plugin/
  .claude-plugin/
    plugin.json
  skills/
    edi-core/SKILL.md          # Identity, RECALL integration, governance
    coding/SKILL.md            # Go coding standards (judgment parts only)
    testing/SKILL.md           # Testing methodology (judgment parts only)
    retrieval-judge/SKILL.md   # RECALL result evaluation
    scaffolding-tests/SKILL.md # Scaffolding test methodology
    refactoring-planning/SKILL.md
    plan-review/SKILL.md
  agents/
    edi-debugger.md
    edi-doc-writer.md
    edi-implementer.md
    edi-researcher.md
    edi-reviewer.md
    edi-test-writer.md
    edi-web-researcher.md
  commands/
    build.md                   # /build (mode switch)
    end.md                     # /end (session capture)
    end-recovery.md
    incident.md
    plan.md
    review.md
    task.md
    ralph.md                   # /ralph (PRD authoring)
  hooks/
    hooks.json                 # PostToolUse, Stop, PreToolUse, SessionStart hooks
    scripts/
      format-go.sh             # gofumpt on *.go files
      protect-files.sh         # Block sensitive file access
      protect-memory.sh        # Block MEMORY.md managed sections
      verify-quality.sh        # golangci-lint + go test + go vet
      generate-briefing.sh     # SessionStart: generate briefing from .edi/
  .mcp.json                    # recall-mcp server configuration
  settings.json                # Default settings (agent: coder)
```

### hooks.json

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/generate-briefing.sh"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/generate-briefing.sh"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/format-go.sh",
            "async": true
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Edit|Write|Read",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/protect-files.sh"
          }
        ]
      },
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/protect-memory.sh"
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/block-dangerous-commands.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/scripts/verify-quality.sh"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Before finishing: Did you complete ALL steps the user requested? Check each requirement against what was actually done. If any step was skipped, partially done, or only described but not implemented, you must go back and complete it. Do not rationalize skipped work as unnecessary."
          }
        ]
      }
    ]
  }
}
```

**Changes from original hooks.json** (informed by Part 8-9 research):

1. **Added `"matcher": "compact"` SessionStart hook** — Re-injects the briefing after context compaction. Without this, EDI's project context is lost during long sessions. (Gap 1, Part 9)
2. **Added `"async": true` to PostToolUse format-go** — Prevents context window pollution from formatting system reminders. Each synchronous formatting event consumes context tokens; async avoids this. (Gap 3, Part 9)
3. **Added `type: "prompt"` Stop hook** — Anti-rationalization gate. Trail of Bits found this catches premature victory declarations. Spotify's equivalent vetoes 25% of agent sessions. (Gap 2, Part 9)

### What Goes Where (for users of AEF)

| Content | Location | Loaded When | Who Authors |
|---|---|---|---|
| AEF methodology (governance, RECALL, identity) | Plugin `skills/` | Always (via plugin) | AEF team |
| Go coding standards (judgment parts) | Plugin `skills/coding/` | When coding | AEF team |
| Formatting enforcement | Plugin `hooks/` PostToolUse | Every file write | AEF team |
| Lint/test enforcement | Plugin `hooks/` Stop | End of task | AEF team |
| Safety guards | Plugin `hooks/` PreToolUse | Before risky actions | AEF team |
| Project-specific context | `CLAUDE.md` in project root | Every session | Project team |
| Project-specific rules | `.claude/rules/*.md` | Per file path | Project team |
| Project profile & history | `.edi/profile.md`, `.edi/history/` | Via SessionStart hook | Project team + EDI |
| RECALL knowledge | MCP server (recall-mcp) | On demand via tool calls | Accumulated over time |

### Migration Path

**Phase 1**: Extract hooks from coding/testing skills into `.claude/settings.json` hooks in this repo. Test that golangci-lint, gofumpt, go test work as hooks alongside existing EDI functionality.

**Phase 2**: Restructure embedded assets into plugin directory layout. Verify all skills, agents, commands work when loaded as a plugin rather than via `--append-system-prompt-file`.

**Phase 3**: Convert SessionStart briefing generation from Go code to a shell script that reads `.edi/` state. This removes the dependency on the `edi` binary for session startup.

**Phase 4**: Publish plugin to a GitHub-hosted marketplace. Update `edi init` to register the marketplace and install the plugin.

**Phase 5**: Make `edi` binary optional for standard workflows. Keep it required only for `edi ralph` and `edi recall` management.

---

## Part 4: What Instructions to REMOVE from Skills

During classification, some instructions stood out as redundant or counter-productive when hooks exist:

### Remove from coding/SKILL.md (replaced by hooks):

These instructions tell the agent to *do something a tool can do deterministically*. With hooks, the tool runs automatically — the agent doesn't need to be told.

| Remove | Replaced By |
|---|---|
| "Run `go fmt ./...` before every commit" | PostToolUse hook: `gofumpt` |
| "Run `go vet ./...` before every commit" | Stop hook: `go vet` |
| "Run `staticcheck ./...` before every commit" | Stop hook: `golangci-lint` (includes staticcheck) |
| "Run `go test -race ./...` before every commit" | Stop hook: `go test -race` |

Removing these from the skill makes it shorter and more focused on *judgment* instructions. The hooks guarantee the tools run — no prompt drift, no context window waste.

### Remove from testing/SKILL.md (replaced by hooks):

| Remove | Replaced By |
|---|---|
| "Always run `go test -race`" | Stop hook |
| "Use `t.Parallel()`" (partially) | `tparallel` linter via Stop hook catches violations |
| "Use `t.Helper()` in helpers" (partially) | `thelper` linter via Stop hook catches violations |

Keep the *explanations* of why these matter (context for when the linter flags something), but remove the "you must do X" imperative — the hook enforces it.

### Remove from edi-core/SKILL.md (replaced by hooks):

| Remove | Replaced By |
|---|---|
| "Do NOT modify EDI-managed sections" (imperative) | PreToolUse hook blocks it mechanically |
| "Keep MEMORY.md under 195 lines" (imperative) | PostToolUse hook checks and blocks |

Keep the *context* about why these limits exist (so the agent understands when the hook fires), but the enforcement is now deterministic.

---

## Part 5: Skill Trimming Opportunities

Beyond hook extraction, some skills could be shorter. The principle: **if Claude already knows it, don't repeat it**.

### coding/SKILL.md candidates for trimming:

| Section | Current Lines | Action | Rationale |
|---|---|---|---|
| Naming conventions (MixedCaps, etc.) | ~15 | Trim to 3 | Claude knows Go naming. Keep only project-specific deviations. |
| Package organization (cmd/, internal/, pkg/) | ~8 | Trim to 2 | Standard Go layout. Claude knows this. |
| Error handling patterns (sentinel, wrapping) | ~12 | Keep 8 | EDI has specific opinions that differ from defaults |
| Anti-patterns list | ~15 | Trim to 6 | Remove ones covered by linters (naked returns, panic, init), keep judgment ones (god packages, global state) |

### testing/SKILL.md candidates for trimming:

| Section | Current Lines | Action | Rationale |
|---|---|---|---|
| Go test essentials (t.Parallel, etc.) | ~15 | Trim to 5 | Claude knows Go testing. Keep only non-obvious ones. |
| Table-driven test example | ~20 | Trim to 8 | Claude knows the pattern. Keep the GIVEN/WHEN/THEN labeling convention. |
| Anti-patterns | ~15 | Trim to 5 | Remove ones covered by linters, keep judgment ones |

**Estimated savings**: ~80 lines across the two skills. This isn't dramatic, but every line removed from a skill is a line that won't be ignored during context pressure.

---

## Part 6: Complete Hook Inventory

### Proposed hooks for AEF plugin:

#### 1. PostToolUse: Auto-format Go files
```bash
#!/bin/bash
# hooks/scripts/format-go.sh
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *.go ]] && command -v gofumpt &>/dev/null; then
  gofumpt -w "$FILE_PATH" 2>/dev/null
fi
exit 0
```

#### 2. PreToolUse: Protect sensitive files
```bash
#!/bin/bash
# hooks/scripts/protect-files.sh
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

PROTECTED_PATTERNS=(".env" "credentials" "secrets" ".key" ".pem")
for pattern in "${PROTECTED_PATTERNS[@]}"; do
  if [[ "$FILE_PATH" == *"$pattern"* ]]; then
    echo "Blocked: Cannot access $FILE_PATH (matches protected pattern '$pattern')" >&2
    exit 2
  fi
done
exit 0
```

#### 3. PreToolUse: Block dangerous Bash commands
```bash
#!/bin/bash
# hooks/scripts/block-dangerous-commands.sh
INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Block destructive patterns
if echo "$COMMAND" | grep -qE 'rm\s+-rf\s+/|git\s+push\s+--force\s+(origin\s+)?(main|master)|git\s+reset\s+--hard'; then
  echo "Blocked: Destructive command detected. Please confirm with user first." >&2
  exit 2
fi
exit 0
```

#### 4. PreToolUse: Protect MEMORY.md managed sections
```bash
#!/bin/bash
# hooks/scripts/protect-memory.sh
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *"MEMORY.md" ]]; then
  OLD_STRING=$(echo "$INPUT" | jq -r '.tool_input.old_string // empty')
  if echo "$OLD_STRING" | grep -qE '^## (Project Quick Reference|Current State|Key Patterns|Known Pitfalls|Key Decisions|Topic Index)'; then
    echo "Blocked: Cannot modify EDI-managed sections of MEMORY.md. Write to 'EDI Observations' section instead." >&2
    exit 2
  fi
fi
exit 0
```

#### 5. Stop: Quality verification
```bash
#!/bin/bash
# hooks/scripts/verify-quality.sh
# Only run if Go files were modified in this session
cd "$CLAUDE_PROJECT_DIR" || exit 0

# Check if any Go files exist
if ! ls *.go */*.go */*/*.go 2>/dev/null | head -1 >/dev/null; then
  exit 0
fi

ERRORS=""

# go vet
if ! go vet ./... 2>&1; then
  ERRORS="${ERRORS}go vet found issues.\n"
fi

# golangci-lint (if available)
if command -v golangci-lint &>/dev/null; then
  if ! golangci-lint run --timeout 60s 2>&1; then
    ERRORS="${ERRORS}golangci-lint found issues.\n"
  fi
fi

# go build
if ! go build ./... 2>&1; then
  ERRORS="${ERRORS}Build failed.\n"
fi

# go test (fast, no race for Stop hook — full race test on commit)
if ! go test -count=1 -short ./... 2>&1; then
  ERRORS="${ERRORS}Tests failed.\n"
fi

if [ -n "$ERRORS" ]; then
  echo "Quality checks failed:" >&2
  echo -e "$ERRORS" >&2
  exit 2
fi

exit 0
```

#### 6. SessionStart: Generate briefing (also used for compaction recovery)

This script fires on both initial session start AND after context compaction (via the `"matcher": "compact"` hook entry). This ensures the EDI briefing is re-injected when Claude Code compresses prior messages, preventing methodology drift during long sessions.

```bash
#!/bin/bash
# hooks/scripts/generate-briefing.sh
# Fires on: SessionStart (initial) AND SessionStart (compact)
# Purpose: Inject/re-inject EDI project context into the session
PROJECT_DIR="$CLAUDE_PROJECT_DIR"
EDI_DIR="$PROJECT_DIR/.edi"

if [ ! -d "$EDI_DIR" ]; then
  exit 0  # Not an EDI project
fi

echo "# EDI Session Briefing"
echo ""

# Project profile
if [ -f "$EDI_DIR/profile.md" ]; then
  echo "## Project Context"
  cat "$EDI_DIR/profile.md"
  echo ""
fi

# Project status
if [ -f "$EDI_DIR/status.md" ]; then
  echo "## Project Status"
  cat "$EDI_DIR/status.md"
  echo ""
fi

# Recent sessions (last 3)
if [ -d "$EDI_DIR/history" ]; then
  echo "## Recent Sessions"
  ls -t "$EDI_DIR/history/"*.md 2>/dev/null | head -3 | while read f; do
    echo "---"
    head -20 "$f"
    echo ""
  done
fi

# Task status
if [ -d "$EDI_DIR/tasks" ]; then
  TOTAL=$(ls "$EDI_DIR/tasks/"*.yaml 2>/dev/null | wc -l)
  if [ "$TOTAL" -gt 0 ]; then
    echo "## Current Tasks ($TOTAL total)"
    for f in "$EDI_DIR/tasks/"*.yaml; do
      TITLE=$(grep '^title:' "$f" | head -1 | sed 's/title: *//')
      STATUS=$(grep '^status:' "$f" | head -1 | sed 's/status: *//')
      echo "- [$STATUS] $TITLE"
    done
    echo ""
  fi
fi

echo "---"
echo "Ready to continue."
exit 0
```

#### 7. Stop: Anti-rationalization gate

A `type: "prompt"` hook that fires when the agent attempts to finish responding. This catches premature victory declarations — the most impactful failure mode in multi-step tasks. Costs one additional API call per task completion.

**Evidence**: Trail of Bits found this essential for security work. Spotify's equivalent LLM judge vetoes 25% of agent sessions for being incomplete or overly ambitious.

**Note**: Known issue — prompt hooks can trigger false positive injection detection (GitHub #17804). Keep the prompt text direct and simple to minimize this risk.

```
Type: prompt (defined in hooks.json, not a script)
Prompt: "Before finishing: Did you complete ALL steps the user requested?
Check each requirement against what was actually done. If any step was
skipped, partially done, or only described but not implemented, you must
go back and complete it. Do not rationalize skipped work as unnecessary."
```

---

## Part 7: Summary Classification Table

| Category | Total | Agent Prompt | Hook | Split | Key Insight |
|---|---|---|---|---|---|
| Identity/Voice | 20 | 20 | 0 | 0 | Persona is holistic, not checkable |
| RECALL Integration | 19 | 17 | 2 | 0 | Query/log decisions need judgment; verification hooks can audit |
| Task Management | 15 | 15 | 0 | 0 | Propagation logic is pure judgment |
| Impl. Governance | 15 | 15 | 0 | 0 | Severity calibration is contextual |
| Auto Memory | 13 | 10 | 3 | 0 | Section protection and line limits are mechanical |
| Go Coding Stds | 48 | 18 | 25 | 5 | **Biggest opportunity** — most formatting/lint rules belong in hooks |
| Testing Standards | 41 | 30 | 8 | 3 | Test methodology stays; tool execution moves to hooks |
| Scaffolding Tests | 65 | 65 | 0 | 0 | Pure methodology, no mechanical parts |
| Refactoring Plan | 33 | 33 | 0 | 0 | Pure planning, no mechanical parts |
| Plan Review | 26 | 26 | 0 | 0 | Pure assessment, no mechanical parts |
| Agent Behaviors | 42 | 42 | 0 | 0 | By definition, agent-level |
| Slash Commands | 94 | 90 | 2 | 2 | /end file-write verification is hookable |
| Subagent Behaviors | 60 | 60 | 0 | 0 | Specialization definitions |
| Ralph Loop | 19 | 19 | 0 | 0 | Escalation logic is judgment |
| **TOTAL** | **510** | **460 (90%)** | **40 (8%)** | **10 (2%)** | Hooks for mechanics, prompts for judgment |

---

## Appendix A: Recommended golangci-lint Configuration

This configuration implements the mechanical Go coding standards that move from skills to hooks:

```yaml
# .golangci.yml
run:
  timeout: 60s
  go: "1.22"

linters:
  disable-all: true
  enable:
    # Error handling (from coding/SKILL.md)
    - errcheck          # Never ignore errors
    - wrapcheck         # Ensure errors are wrapped with context
    - errorlint         # Correct usage of errors.Is/As

    # Static analysis (from coding/SKILL.md)
    - govet             # Go vet checks
    - staticcheck       # Real bug detection
    - gocritic          # Anti-patterns (init, panic, etc.)

    # Style enforcement (from coding/SKILL.md)
    - nakedret          # No naked returns
    - revive            # Naming, docs, conventions
    - gofumpt           # Strict formatting

    # Safety (from coding/SKILL.md)
    - gosec             # Security issues
    - gochecknoglobals  # No global mutable state

    # Testing (from testing/SKILL.md)
    - tparallel         # t.Parallel() usage
    - thelper           # t.Helper() in test helpers

    # Code quality
    - unused            # Unused code detection
    - ineffassign       # Ineffectual assignments
    - typecheck         # Type checking

linters-settings:
  nakedret:
    max-func-lines: 0   # Forbid all naked returns

  revive:
    rules:
      - name: exported
        severity: warning
        arguments:
          - "checkPrivateReceivers"
      - name: var-naming
        severity: warning
      - name: package-comments
        severity: warning

  gocritic:
    enabled-checks:
      - initClause
      - exitAfterDefer
      - dupImport

  wrapcheck:
    ignoreSigs:
      - .Errorf(   # fmt.Errorf is wrapping
      - errors.New(
      - errors.Unwrap(

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

---

## Appendix B: Plugin Manifest

```json
{
  "name": "aef",
  "description": "AI Engineering Framework — methodology, knowledge integration, and quality enforcement for Claude Code",
  "version": "0.1.0",
  "author": {
    "name": "AEF Team"
  },
  "repository": "https://github.com/your-org/aef-plugin",
  "license": "MIT",
  "keywords": ["engineering", "methodology", "recall", "coding-standards", "testing"]
}
```

---

## Part 8: Evidence Base — What Research Says About This Plan

> Added 2026-02-24 based on comprehensive research across academic studies, benchmark data, Anthropic internal practices, community surveys, and production systems.

### 8.1 The Core Thesis Is Validated

The 90% agent-prompt / 8% hook / 2% split classification is strongly supported by evidence:

| Evidence | Source | What It Validates |
|---|---|---|
| AI-generated code has **1.7x more issues** than human code | Qodo 2025 (600+ developers) | Hooks are necessary — prompts alone don't prevent defects |
| Verification loops improve pass@1 by **9.2%** | LLMLOOP, ICSME 2025 | Feedback loops measurably improve output quality |
| Structured feedback reduces critical bugs by **37-40%** | IEEE 2025 | Formal verification beats ad-hoc checking |
| Scaffold choice alone swings SWE-bench by **25.6 percentage points** | Vals.ai analysis | The harness matters as much as the model |
| **94%** of LLM compilation errors are type-related | ETH Zurich / UC Berkeley 2025 | Type checkers (Go compiler, `go vet`) are the highest-ROI automated check |
| Semgrep catches IDORs at **61% precision** vs Claude alone at **22%** | Semgrep research program | Security scanners catch what prompts miss |
| Mechanical TDD enforcement produces poorly coupled code | Nizar's controlled experiment, 2025 | Judgment skills (governance, plan-review) cannot be replaced by hooks |
| Hooks increased skill activation from **~20% to ~84%** | alexop.dev TDD experiment | Hooks dramatically improve protocol adherence |

### 8.2 Anthropic's Own Verification Hierarchy

Anthropic's official best practices rank verification approaches:

1. **Deterministic tools** (highest confidence): linters, type checkers, test suites
2. **Test execution**: running existing tests to verify behavior
3. **CLAUDE.md instructions**: system prompt guidance
4. **LLM-as-judge**: agent-type hooks that evaluate output (weakest)

Boris Cherny (Claude Code creator): *"Give Claude a way to verify its work. If Claude has that feedback loop, it will 2-3x the quality of the final result."*

This hierarchy validates the plan's approach: deterministic tools in hooks (golangci-lint, go test, gofumpt), judgment in prompts (governance, RECALL, severity calibration).

### 8.3 SWE-bench Progression Validates Agentic Verification

The progression from 2% to 79% on SWE-bench Verified was driven primarily by agentic verification loops, not model improvements:

- **2023**: ~2% resolve rate (early RAG scaffolds)
- **2024**: 49% (Claude 3.5 Sonnet with self-correction loops)
- **2025-2026**: 79.2% (Claude Opus 4.6 with thinking + test execution loops)

Key insight from Anthropic's SWE-bench agent: it checks its work by **re-running the reproduction script and the project's existing tests** to validate each fix. This is exactly what the verify-quality.sh Stop hook implements.

### 8.4 What Production Systems Look Like

The most sophisticated verification systems in the wild:

| System | Architecture | Key Innovation |
|---|---|---|
| **Claude Pilot** | 15-hook pipeline | Compaction persistence, spec guards, context monitoring |
| **Trail of Bits** | Anti-rationalization gates | Stop prompt hook catches premature victory declaration |
| **Spotify Honk** | Two-layer verification | Deterministic verifiers + LLM judge that vetoes **25% of sessions** |
| **alexop.dev TDD** | Subagent isolation | Separate subagents per TDD phase to prevent context pollution |

AEF's plan exceeds all of these in the judgment/methodology layer (RECALL, governance, plan-review, retrieval-judge). The gap is entirely in the mechanical enforcement layer — which this plan's hooks address.

### 8.5 Key Quantitative Findings

**AI code quality deficit** (Qodo 2025, 600+ developers):

| Metric | AI vs Human Code |
|---|---|
| Total issues | 1.7x more in AI-generated code |
| Maintainability errors | 1.64x more |
| Logic/correctness errors | 1.75x more |
| Security findings | 1.57x more |

**Code churn** (GitClear 2025, 211 million changed lines):

- Code duplication grew **4x** during 2024
- Blocks with 5+ duplicated lines increased **8x**
- Refactoring declined from 25% to under 10% of changed lines
- New code revised within two weeks grew from 3.1% to 5.7%

**Productivity reality** (METR RCT, 16 developers, 246 tasks):

- Developers with AI access were **19% slower** (not faster)
- Developers *believed* they were **20% faster** (39 percentage point perception gap)
- Explanation: AI struggles with deep implicit context and strict quality standards
- Less rigorous studies find 16-55% improvements in different contexts

The METR finding is particularly relevant to AEF: EDI's methodology (governance, RECALL, scaffolding tests) provides exactly the "implicit context and strict quality standards" that makes AI effective in mature codebases.

### 8.6 Sources

- [Qodo 2025 State of AI Code Quality Report](https://www.qodo.ai/reports/state-of-ai-code-quality/)
- [GitClear AI Copilot Code Quality 2025](https://www.gitclear.com/ai_assistant_code_quality_2025_research)
- [LLMLOOP (ICSME 2025)](https://ieeexplore.ieee.org/document/11185878/)
- [ETH Zurich/UC Berkeley Type Safety Study](https://medium.com/@michaelhenderson/how-type-safety-catches-94-of-llm-code-errors-db63337a1478)
- [Semgrep AI-Powered Detection](https://semgrep.dev/blog/2025/ai-powered-detection-with-semgrep/)
- [Epoch AI SWE-bench Verified Leaderboard](https://epoch.ai/benchmarks/swe-bench-verified)
- [Vals.ai SWE-bench Analysis](https://www.vals.ai/benchmarks/swebench)
- [Anthropic SWE-bench Performance](https://www.anthropic.com/research/swe-bench-sonnet)
- [Verdent SWE-bench Technical Report](https://www.verdent.ai/blog/swe-bench-verified-technical-report)
- [METR RCT: AI Developer Productivity Study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)
- [How Anthropic Teams Use Claude Code](https://claude.com/blog/how-anthropic-teams-use-claude-code)
- [Microsoft AI Code Review at Scale](https://devblogs.microsoft.com/engineering-at-microsoft/enhancing-code-quality-at-scale-with-ai-powered-code-reviews/)
- [Spotify Honk Feedback Loops](https://engineering.atspotify.com/2025/12/feedback-loops-background-coding-agents-part-3/)
- [arXiv: Assessing Quality and Security of AI-Generated Code](https://arxiv.org/abs/2508.14727)
- [arXiv: LLMs vs Static Analysis for Vulnerability Detection](https://arxiv.org/html/2508.04448v1)
- [Stack Overflow 2025 Developer Survey](https://survey.stackoverflow.co/2025/)

---

## Part 9: Gap Analysis — What the Plan Is Missing

> Based on research findings compared against Parts 1-7 of this document.

### Gap 1: Compaction Persistence (CRITICAL)

**Problem**: Claude Code compresses prior messages as conversations approach context limits. When this happens, the EDI briefing (injected via SessionStart hook) gets summarized or lost. CLAUDE.md content is wrapped in framing that says it *"may or may not be relevant"* — under context pressure, carefully written standards get deprioritized.

**Evidence**: Claude Pilot solves this with `pre_compact.py` (saves state before compression) and `post_compact_restore.py` (re-injects critical context after). The plan's SessionStart hook (Part 6, §6) fires only at launch — it has no `compact` matcher to re-inject after compaction.

**Fix**: Add a second SessionStart hook entry with `"matcher": "compact"` that re-injects the briefing. This is a ~5-line addition to hooks.json.

### Gap 2: Anti-Rationalization Gate (HIGH)

**Problem**: Claude can declare victory prematurely — "I've completed the implementation" when critical steps were skipped. The plan's Stop hook (verify-quality.sh) checks mechanical quality but doesn't ask "did you actually finish what was requested?"

**Evidence**: Trail of Bits uses a `type: "prompt"` Stop hook for this. Claude itself acknowledged (GitHub Issue #26761): *"Adding more rules is useless if I always do the first thing I see I can do."* Spotify's LLM judge vetoes **25% of agent sessions** for being too ambitious or incomplete.

**Fix**: Add a `type: "prompt"` Stop hook that asks whether acceptance criteria were met. This costs one additional API call per task completion but catches the highest-impact failure mode.

### Gap 3: PostToolUse Formatter Context Pollution (MEDIUM)

**Problem**: The plan puts `gofumpt` as a synchronous PostToolUse hook (Part 6, §1). Every formatting change generates a system reminder that consumes context window space. In monorepos, one developer reported **180 seconds of delay per interaction** from sequential hooks.

**Evidence**: Lakshmi Narasimhan's experience: *"Every formatting change generates a system reminder that pollutes your context window."* Claude Code added `async: true` support in January 2026 specifically to address this.

**Fix**: Either add `"async": true` to the format-go hook, or remove the PostToolUse formatter entirely — the Stop hook's golangci-lint already includes `gofumpt` checking. Formatting can happen once at task end rather than on every edit.

### Gap 4: Context Usage Monitoring (MEDIUM)

**Problem**: AEF sessions are context-heavy (briefings, skill content, RECALL results). No mechanism warns when context pressure may cause instruction degradation.

**Evidence**: Claude Pilot tracks context usage and warns at 80% and 90%. Research shows larger context windows can degrade performance by **24.2%** when filled with irrelevant tokens.

**Fix**: A lightweight PostToolUse hook that estimates context usage, or an agent-type hook at key checkpoints. Lower priority than Gaps 1-2.

### Gap 5: Known Hook System Bugs (LOW — awareness only)

The plan assumes hooks work as documented. Known issues to be aware of:

| Issue | Impact | Workaround |
|---|---|---|
| Stop hooks in SKILL.md frontmatter don't fire (#19225) | Stop hooks must be in settings.json or hooks.json | Already correct in this plan |
| PreToolUse hooks sometimes don't block (#2814) | Safety guards may be bypassed | Add agent prompt as fallback |
| Prompt hooks can trigger false injection detection (#17804) | Anti-rationalization hook may be flagged | Keep prompt text simple and direct |
| Stop hooks hang on stalled API calls (superpowers #390) | Agent-type hooks need timeout | Add timeout to any agent-type hooks |

### Gap 6: Deferred Plugin Architecture

The plan proposes restructuring AEF as a Claude Code plugin (Part 3). No evidence of a mature plugin marketplace or standardized plugin format exists yet. The migration path (5 phases) is sound but depends on ecosystem maturity.

**Recommendation**: Implement Phase 1 (hooks in `.claude/settings.json`) immediately. Defer Phases 2-5 until the plugin ecosystem stabilizes.

---

## Part 10: Revised Implementation Priority

> Incorporating research findings with the original plan.

### Priority 1: Implement Now (validated by evidence, in the plan)

| Item | Plan Reference | Evidence Support |
|---|---|---|
| `.golangci.yml` configuration | Appendix A | golangci-lint catches the exact bug classes AI introduces most |
| Stop hook: `verify-quality.sh` | Part 6, §5 | SWE-bench agents use test execution loops; 9.2% pass@1 improvement from feedback |
| PreToolUse: `protect-files.sh` | Part 6, §2 | 40% of AI-generated code contains security vulnerabilities (Semgrep) |
| PreToolUse: `protect-memory.sh` | Part 6, §4 | Deterministic enforcement ranked highest in Anthropic's hierarchy |
| PreToolUse: `block-dangerous-commands.sh` | Part 6, §3 | Safety guard — standard practice in all production systems |
| SessionStart: `generate-briefing.sh` | Part 6, §6 | Anthropic teams that documented expectations in CLAUDE.md saw best results |
| Remove lint imperatives from skills | Part 4 | Boris Cherny's team does this — hooks enforce, prompts explain |

### Priority 2: Add to Plan (missing, high evidence)

| Item | Evidence | Effort |
|---|---|---|
| SessionStart hook with `"matcher": "compact"` | Claude Pilot compaction persistence | ~5 lines in hooks.json + reuse existing briefing script |
| Stop hook `type: "prompt"` (anti-rationalization) | Trail of Bits; Spotify vetoes 25% of sessions | ~10 lines in hooks.json |
| PostToolUse format-go: add `"async": true"` or remove | Context pollution evidence | 1-line change |

### Priority 3: Defer (in plan but lower priority)

| Item | Reason to Defer |
|---|---|
| RECALL verification Stop hook (agent-type) | LLM-as-judge is weakest verification; keep as prompt instead |
| `/end` file-write verification hooks | Low ROI edge case |
| Plugin architecture (Parts 3, Phase 2-5) | Ecosystem not mature enough |
| Context usage monitoring | Useful but complex; wait for native support |

### Scorecard: AEF Position After Implementation

| Dimension | Best in Class | AEF After Plan | Gap |
|---|---|---|---|
| Methodology / Judgment Skills | No comparable system exists | **10/10** | AEF leads |
| Knowledge Integration (RECALL) | No comparable system exists | **10/10** | AEF leads |
| Mechanical Verification (hooks) | Claude Pilot (15 hooks) | **7/10** | Priority 1+2 closes most distance |
| Compaction Persistence | Claude Pilot | **6/10** | Compact matcher addresses core case |
| Safety Guards | Trail of Bits | **8/10** | PreToolUse hooks cover primary risks |
| Anti-Rationalization | Trail of Bits | **7/10** | Prompt Stop hook covers primary case |
| Distribution | Plugin ecosystem | **3/10** | Deferred — wait for ecosystem |
| Observability | disler (multi-agent) | **4/10** | Flight recorder partially covers |

**Overall**: AEF has the deepest methodology of any system in the field. The gap is entirely in mechanical enforcement, and this plan closes most of that distance. The judgment layer — governance, RECALL, retrieval-judge, plan-review, scaffolding tests — is where AEF's genuine differentiation lives, and no other system approaches it.

---

## Appendix C: Verification Tooling Sources

| Tool | Language | Role | Source |
|---|---|---|---|
| gofumpt | Go | Formatter (PostToolUse) | https://github.com/mvdan/gofumpt |
| golangci-lint | Go | Meta-linter (Stop) | https://golangci-lint.run/ |
| Biome | JS/TS | Formatter + Linter | https://biomejs.dev/ |
| Ruff | Python | Linter + Formatter | https://docs.astral.sh/ruff/ |
| Prettier | JS/TS/CSS | Formatter | https://prettier.io/ |
| Semgrep | Multi | SAST | https://semgrep.dev/ |
| Trivy | Multi | Vuln scanner | https://trivy.dev/ |
| Pyright | Python | Type checker | https://github.com/microsoft/pyright |
| Vitest | JS/TS | Test runner | https://vitest.dev/ |
| cargo-nextest | Rust | Test runner | https://nexte.st/ |
