# Auto Memory Alignment Specification

**Status**: Implemented (Phases 1 + 3) — Revised to merge-based co-ownership model
**Created**: February 9, 2026
**Revised**: February 19, 2026
**Author**: EDI Analysis
**Version**: 1.1

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Analysis](#2-current-state-analysis)
3. [Gap Analysis: EDI RECALL vs Claude Auto Memory](#3-gap-analysis-edi-recall-vs-claude-auto-memory)
4. [Alignment Strategy](#4-alignment-strategy)
5. [Implementation Plan](#5-implementation-plan)
6. [Specification: Briefing-to-MEMORY.md Bridge](#6-specification-briefing-to-memorymd-bridge)
7. [Specification: RECALL-to-Auto-Memory Sync](#7-specification-recall-to-auto-memory-sync)
8. [Specification: Session End Memory Capture](#8-specification-session-end-memory-capture)
9. [Specification: Memory-Aware Context Generation](#9-specification-memory-aware-context-generation)
10. [Specification: Enhanced Skill Integration](#10-specification-enhanced-skill-integration)
11. [Risk Analysis](#11-risk-analysis)
12. [Success Criteria](#12-success-criteria)

---

## 1. Executive Summary

### Problem

Claude Code now ships with **auto memory** — a built-in persistent memory stored at `~/.claude/projects/<project>/memory/MEMORY.md` that is automatically loaded into the system prompt. EDI (AEF's harness) independently implements a parallel memory/context system through RECALL (MCP-based knowledge search), session briefings (`.edi/profile.md`, `.edi/status.md`, `.edi/history/`), and flight recorder logs.

These two systems currently operate in isolation:

| Capability | Claude Auto Memory | EDI RECALL + Briefing |
|---|---|---|
| **Persistence** | `MEMORY.md` in `~/.claude/projects/` | SQLite DB + `.edi/` files |
| **Injection** | Automatic (system prompt) | Manual (system prompt file via `--append-system-prompt-file`) |
| **Granularity** | Free-form markdown (<200 lines) | Typed knowledge (pattern/decision/failure) + structured briefings |
| **Retrieval** | Always loaded (full file) | On-demand search (recall_search MCP tool) |
| **Capture** | Agent writes via Edit/Write tools | Prompted at session end via `/end` + `recall_add` |
| **Scope** | Per-project directory | Global + project scoped |
| **Cross-session** | Yes (file persists) | Yes (DB persists) |

The isolation creates three problems:
1. **Redundant context** — Both systems inject project context, wasting tokens
2. **Split knowledge** — Some insights live in MEMORY.md, others in RECALL; neither is complete
3. **Missed synergy** — RECALL's structured knowledge could seed MEMORY.md; auto memory's always-on nature could surface RECALL's best insights without requiring explicit search

### Solution

Align EDI with Claude's auto memory to create a **layered memory architecture**:

```
LAYER 1: Auto Memory (MEMORY.md)       ← Always loaded, high-signal summary
    ↕ sync
LAYER 2: EDI Briefing (profile+status)  ← Session-start context, project state
    ↕ enrichment
LAYER 3: RECALL Knowledge Base          ← Deep searchable knowledge, typed items
```

**Key principle**: Auto memory becomes the **cache layer** for RECALL's highest-value insights. EDI manages what gets promoted to MEMORY.md and keeps it current.

---

## 2. Current State Analysis

### EDI's Memory Stack (Existing)

```
SESSION START
├── .edi/profile.md          → Project overview (manual, static)
├── .edi/status.md           → Current milestone (updated at /end)
├── .edi/history/*.md        → Recent session summaries
├── .edi/tasks/active.yaml   → Active task list
└── RECALL DB                → Searchable knowledge items
    ├── Patterns             → Reusable techniques
    ├── Decisions            → Choices with rationale
    ├── Failures             → What went wrong + fix
    └── Context              → Domain knowledge

DURING SESSION
├── recall_search            → On-demand knowledge retrieval
├── flight_recorder_log      → Decision/error/milestone logging
└── Agent system prompts     → Mode-specific instructions

SESSION END (/end)
├── recall_add               → Capture new knowledge
├── .edi/status.md           → Update project status
└── .edi/history/*.md        → Save session summary
```

### Claude Auto Memory (New Built-in)

```
ALWAYS LOADED
└── ~/.claude/projects/<project>/memory/MEMORY.md
    ├── Key learnings from past sessions
    ├── Project patterns and conventions
    ├── Common mistakes to avoid
    └── Links to detailed topic files (e.g., debugging.md)

TOPIC FILES (Optional)
└── ~/.claude/projects/<project>/memory/*.md
    ├── debugging.md
    ├── patterns.md
    └── architecture.md
```

### How EDI Currently Launches Claude Code

From `edi/internal/cli/launch.go` and `edi/internal/launch/context.go`:

1. Load config from `.edi/config.yaml`
2. Sync tasks from Claude Code
3. Generate briefing from profile + history + status + tasks
4. Write `.mcp.json` for RECALL server
5. Build context file (agent prompt + briefing + RECALL instructions)
6. `syscall.Exec()` to replace process with Claude Code + `--append-system-prompt-file`

The context file goes into `~/.edi/cache/session-*.md`.

---

## 3. Gap Analysis: EDI RECALL vs Claude Auto Memory

### Overlap

| Feature | EDI | Auto Memory | Conflict? |
|---|---|---|---|
| Project description | `.edi/profile.md` | MEMORY.md "project overview" section | Yes — duplicated |
| Coding conventions | Skills (coding.md) | MEMORY.md conventions section | Partial — skills are more structured |
| Past decisions | RECALL `decision` type | Could be in MEMORY.md | No — different depth |
| Known failures | RECALL `failure` type | Could be in MEMORY.md | No — different depth |
| Session continuity | `.edi/history/` + `.edi/status.md` | MEMORY.md "current state" section | Yes — duplicated |

### Gaps in Each System

**What EDI has that auto memory lacks:**
- Typed, searchable knowledge with scopes (global vs project)
- Structured capture workflow with templates
- Flight recorder for session audit trail
- Agent-specific behaviors and skills
- Task management with RECALL enrichment
- Session lifecycle management (stale detection, recovery)

**What auto memory has that EDI lacks:**
- Zero-latency context (always in system prompt, no MCP call needed)
- Topic-based organization with linked files
- Native integration (no harness needed, works outside EDI)
- Concise format — forces distillation to highest-signal content
- Self-updating — Claude writes to it naturally during sessions

### Key Insight

Auto memory and RECALL are **complementary, not competing**:
- **MEMORY.md** = L1 cache (fast, small, always available)
- **RECALL** = L2 storage (large, searchable, typed, on-demand)

The best architecture uses MEMORY.md as a **curated index** pointing to RECALL's deeper knowledge, with EDI managing the promotion pipeline.

---

## 4. Alignment Strategy

### 4.1 Layer Architecture

```
┌───────────────────────────────────────────────────────────┐
│  LAYER 1: MEMORY.md (Always Loaded)                       │
│  ← ~150 lines, highest-signal content                     │
│                                                           │
│  Sections:                                                │
│  - Project Quick Reference (from .edi/profile.md)         │
│  - Current State (from .edi/status.md)                    │
│  - Top Patterns (promoted from RECALL)                    │
│  - Known Pitfalls (promoted from RECALL failures)         │
│  - Key Decisions (promoted from RECALL decisions)         │
│  - Topic Index (links to memory/*.md files)               │
│                                                           │
│  Updated by: EDI (merge) at session start + session end   │
│  Co-owned: Claude auto-writes preserved across merges     │
└───────────┬───────────────────────────────────────────────┘
            │ sync/promote
┌───────────▼───────────────────────────────────────────────┐
│  LAYER 2: EDI Session Context                             │
│  ← System prompt via --append-system-prompt-file          │
│                                                           │
│  Contains:                                                │
│  - Agent mode and instructions                            │
│  - Session ID and metadata                                │
│  - RECALL tool instructions                               │
│  - Slash command docs                                     │
│  - Recent history (if not in MEMORY.md)                   │
│  - Active tasks                                           │
│                                                           │
│  NO LONGER contains: profile, status (now in MEMORY.md)   │
└───────────┬───────────────────────────────────────────────┘
            │ search/retrieve
┌───────────▼───────────────────────────────────────────────┐
│  LAYER 3: RECALL Knowledge Base                           │
│  ← On-demand via MCP tools                                │
│                                                           │
│  Contains (unchanged):                                    │
│  - All knowledge items (pattern, decision, failure, etc.) │
│  - Full-text + vector search                              │
│  - Feedback signals and usefulness scoring                │
│  - Flight recorder logs                                   │
│                                                           │
│  NEW: Tracks which items are promoted to MEMORY.md        │
└───────────────────────────────────────────────────────────┘
```

### 4.2 Design Principles

1. **MEMORY.md is co-managed** — EDI manages structured sections (project context, promoted RECALL items); Claude can freely write to other sections (EDI Observations, auto-saved memories). EDI uses merge-based updates that preserve non-EDI content.
2. **Deduplication over duplication** — Move profile and status content from EDI's system prompt to MEMORY.md; don't have both
3. **Promote, don't copy** — RECALL items are promoted to MEMORY.md when they prove high-value (high usefulness score, frequently retrieved)
4. **Topic files for depth** — Use `memory/*.md` topic files for detailed knowledge that doesn't fit in 200-line MEMORY.md
5. **Backward compatible** — EDI should work with or without auto memory; degrade gracefully
6. **Never overwrite** — EDI reads existing MEMORY.md before writing, parses sections, and preserves anything it doesn't manage. This prevents destroying Claude's auto-saved memories.

### 4.3 What Changes

| Component | Current | Proposed |
|---|---|---|
| **Briefing generator** | Renders profile+status+history into context | Writes profile+status to MEMORY.md; renders only agent+tasks+RECALL instructions into context |
| **`/end` command** | Captures to RECALL only | Also updates MEMORY.md with promoted insights |
| **MEMORY.md** | Not managed by EDI | Co-managed: EDI owns structured sections via merge; Claude's auto-writes preserved |
| **Context file** | Contains everything | Contains only session-specific content (agent, tasks, RECALL, commands) |
| **RECALL items** | No promotion tracking | New `promoted_to_memory` field |
| **`edi init`** | Creates `.edi/` structure | Also seeds initial MEMORY.md from profile |
| **Topic files** | Don't exist | Created from RECALL high-value items by domain |

### 4.4 What Does NOT Change

- RECALL MCP server (no changes to tools or storage)
- Agent definitions (markdown files with frontmatter)
- Skills (installed to `~/.claude/skills/`)
- Flight recorder (logging behavior unchanged)
- Session lifecycle (stale detection, recovery)
- Task system (active.yaml, sync, annotations)
- Ralph loop (autonomous execution)

---

## 5. Implementation Plan

### Phase 1: MEMORY.md Generation (Core Bridge)

**Goal**: EDI generates and manages MEMORY.md from existing `.edi/` data.

**Changes:**
1. New `internal/memory/` package — generates MEMORY.md content
2. Modify `internal/cli/launch.go` — call memory generator on launch
3. Modify `internal/cli/init.go` — seed initial MEMORY.md during `edi init`
4. New `internal/memory/topics.go` — manage topic files

**Files:**
- `edi/internal/memory/generator.go` (new)
- `edi/internal/memory/topics.go` (new)
- `edi/internal/memory/sync.go` (new)
- `edi/internal/cli/launch.go` (modify)
- `edi/internal/cli/init.go` (modify)

### Phase 2: Context Deduplication

**Goal**: Remove profile/status from system prompt context, since it's now in MEMORY.md.

**Changes:**
1. Modify `internal/briefing/generator.go` — skip profile/status when MEMORY.md is active
2. Modify `internal/launch/context.go` — slimmer context file
3. Add config option: `memory.auto_memory_enabled: true` (default when detected)

**Files:**
- `edi/internal/briefing/generator.go` (modify)
- `edi/internal/launch/context.go` (modify)
- `edi/internal/config/schema.go` (modify)

### Phase 3: Session End Memory Updates

**Goal**: `/end` command updates MEMORY.md with session insights.

**Changes:**
1. Modify `/end` command — add MEMORY.md update step
2. Add promotion logic — identify RECALL items worth surfacing in MEMORY.md
3. Add topic file generation — group related knowledge into topic files

**Files:**
- `edi/internal/assets/commands/end.md` (modify)
- `edi/internal/memory/promote.go` (new)

### Phase 4: RECALL Promotion Tracking

**Goal**: Track which RECALL items are promoted to MEMORY.md.

**Changes:**
1. Add `promoted_to_memory` field to RECALL items
2. Add `edi memory` CLI command for manual management
3. Add `memory sync` subcommand for explicit resync

**Files:**
- `edi/internal/recall/storage.go` (modify — if v0)
- `codex/internal/storage/metadata.go` (modify — if codex)
- `edi/internal/cli/memory.go` (new)

---

## 6. Specification: Briefing-to-MEMORY.md Bridge

### Memory Directory Detection

```go
// DetectAutoMemoryDir finds the Claude auto memory directory for the current project
func DetectAutoMemoryDir() (string, error) {
    home, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()

    // Claude Code stores auto memory at:
    // ~/.claude/projects/<sanitized-project-path>/memory/
    sanitized := sanitizePath(cwd) // replaces / with -
    memDir := filepath.Join(home, ".claude", "projects", sanitized, "memory")

    if _, err := os.Stat(memDir); os.IsNotExist(err) {
        return "", fmt.Errorf("auto memory directory not found: %s", memDir)
    }
    return memDir, nil
}
```

### MEMORY.md Structure

```markdown
# AEF Project Memory

## Project Quick Reference
<!-- Synced from .edi/profile.md -->
[Concise project description, tech stack, conventions]

## Current State
<!-- Synced from .edi/status.md -->
[Current milestone, recent completions, next steps]

## Key Patterns
<!-- Promoted from RECALL patterns with high usefulness -->
- [Pattern 1]: [one-line description]
- [Pattern 2]: [one-line description]

## Known Pitfalls
<!-- Promoted from RECALL failures -->
- [Pitfall 1]: [symptom] -> [fix]
- [Pitfall 2]: [symptom] -> [fix]

## Key Decisions
<!-- Promoted from RECALL decisions -->
- [Decision 1]: [choice] because [rationale]
- [Decision 2]: [choice] because [rationale]

## Topic Index
- [architecture.md](architecture.md) - System architecture decisions
- [debugging.md](debugging.md) - Common debugging patterns
```

### Generation Logic

```go
func GenerateMemory(cfg *config.Config, projectPath string) (string, error) {
    var sb strings.Builder

    sb.WriteString("# Project Memory\n\n")

    // Section 1: Quick Reference (from profile, condensed)
    if profile, err := loadProfile(projectPath); err == nil {
        sb.WriteString("## Project Quick Reference\n")
        sb.WriteString(condense(profile, 30)) // max 30 lines
        sb.WriteString("\n\n")
    }

    // Section 2: Current State (from status)
    if status, err := loadStatus(projectPath); err == nil {
        sb.WriteString("## Current State\n")
        sb.WriteString(status)
        sb.WriteString("\n\n")
    }

    // Section 3-5: Promoted RECALL items
    if cfg.Recall.Enabled {
        if items, err := getPromotedItems(cfg); err == nil {
            sb.WriteString(renderPromotedItems(items))
        }
    }

    // Section 6: Topic index
    if topics, err := listTopicFiles(memDir); err == nil && len(topics) > 0 {
        sb.WriteString("## Topic Index\n")
        for _, t := range topics {
            sb.WriteString(fmt.Sprintf("- [%s](%s) - %s\n", t.Name, t.File, t.Description))
        }
    }

    return sb.String(), nil
}
```

### Line Budget

MEMORY.md has a 200-line limit (lines after 200 truncated in system prompt).

| Section | Budget | Source |
|---|---|---|
| Project Quick Reference | 30 lines | `.edi/profile.md` (condensed) |
| Current State | 15 lines | `.edi/status.md` |
| Key Patterns | 20 lines | RECALL patterns (top 10) |
| Known Pitfalls | 20 lines | RECALL failures (top 10) |
| Key Decisions | 20 lines | RECALL decisions (top 10) |
| Topic Index | 10 lines | Links to topic files |
| Headers + spacing | 15 lines | Structural |
| **Buffer** | **70 lines** | Reserved for growth |

---

## 7. Specification: RECALL-to-Auto-Memory Sync

### Promotion Criteria

A RECALL item is eligible for promotion to MEMORY.md when:

```go
type PromotionCriteria struct {
    MinUsefulnessRate float64 // >= 0.6 (60% useful feedback)
    MinRetrievalCount int     // >= 3 (retrieved at least 3 times)
    MaxAge            int     // <= 90 days old
    AllowedTypes      []string // ["pattern", "decision", "failure"]
}
```

### Sync Flow

```
edi launch
    │
    ├── Load .edi/profile.md
    ├── Load .edi/status.md
    ├── Query RECALL for promoted items
    │   └── SELECT * FROM items
    │       WHERE promoted_to_memory = TRUE
    │       OR (usefulness_score >= 0.6
    │           AND retrieval_count >= 3
    │           AND type IN ('pattern', 'decision', 'failure'))
    │       ORDER BY usefulness_score DESC
    │       LIMIT 30
    │
    ├── Generate MEMORY.md content
    ├── Write to auto memory directory
    └── Continue with slimmed context generation
```

### Topic File Generation

For domains with 5+ RECALL items, generate a topic file:

```go
func GenerateTopicFile(domain string, items []RecallItem) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("# %s\n\n", titleCase(domain)))

    // Group by type
    for _, itemType := range []string{"decision", "pattern", "failure"} {
        typed := filterByType(items, itemType)
        if len(typed) == 0 { continue }

        sb.WriteString(fmt.Sprintf("## %ss\n\n", titleCase(itemType)))
        for _, item := range typed {
            sb.WriteString(fmt.Sprintf("### %s\n", item.Title))
            sb.WriteString(item.Content)
            sb.WriteString("\n\n")
        }
    }
    return sb.String()
}
```

---

## 8. Specification: Session End Memory Capture

### Modified `/end` Flow

Current `/end` command steps 1-8 remain unchanged. New step inserted between 5 and 6:

**Step 5.5: Update MEMORY.md**

```markdown
5.5 **Update** MEMORY.md with session insights:
    - Read current MEMORY.md
    - Update "Current State" section from new .edi/status.md
    - Add any newly captured RECALL items that meet promotion criteria
    - Remove items flagged as outdated
    - Ensure total stays under 200 lines
    - Write updated MEMORY.md

    Present changes to user:
    ```
    MEMORY.md updates:
    + Added pattern: "Exponential backoff with jitter for retries"
    + Added failure: "Race condition in token refresh"
    ~ Updated: Current State section
    - Removed: Outdated payment API reference

    Apply? [Y]es / [E]dit / [S]kip
    ```
```

### Capture-to-Memory Pipeline

```
/end triggered
    │
    ├── Step 1-4: Summarize session (unchanged)
    │
    ├── Step 5: Capture to RECALL (unchanged)
    │   └── recall_add({type, title, content, tags})
    │
    ├── Step 5.5: Update MEMORY.md (NEW)
    │   ├── Read current MEMORY.md
    │   ├── Determine which new captures qualify for promotion
    │   │   └── Decisions and failures always qualify
    │   │   └── Patterns qualify if user confirms
    │   ├── Update Current State from status.md
    │   ├── Render updated MEMORY.md
    │   └── Write to auto memory directory
    │
    ├── Step 6: Update .edi/status.md (unchanged)
    │
    ├── Step 7: Write session history (unchanged)
    │
    └── Step 8: Confirm (enhanced)
        ```
        Session saved to .edi/history/2026-02-09-abc12345.md
        Captured 2 items to RECALL.
        Updated MEMORY.md with 2 new entries.
        ```
```

---

## 9. Specification: Memory-Aware Context Generation

### Current Context File Contents

```
# EDI - Enhanced Development Intelligence
Session ID: ...
Started: ...

## Current Mode: coder
[Agent system prompt]

## Session Briefing
# EDI Briefing: project-name
## Project Context        ← REDUNDANT with MEMORY.md
[full profile.md]
## Project Status         ← REDUNDANT with MEMORY.md
[full status.md]
## Recent Sessions
[history entries]
## Current Tasks
[task list]

## RECALL Knowledge Base
[tool instructions]

## EDI Slash Commands
[command docs]
```

### Proposed Context File (Memory-Aware)

When auto memory is detected and managed by EDI:

```
# EDI - Enhanced Development Intelligence
Session ID: ...
Started: ...

## Current Mode: coder
[Agent system prompt]

## Session Context
[NOTE: Project context and status are in MEMORY.md (auto-loaded)]

### Recent Sessions
[history entries — only if not already captured in MEMORY.md]

### Current Tasks
[task list]

## RECALL Knowledge Base
[tool instructions]
MEMORY.md contains promoted RECALL items. For deeper search, use recall_search.

## EDI Slash Commands
[command docs]
```

### Token Savings Estimate

| Section | Current (tokens) | Proposed (tokens) | Savings |
|---|---|---|---|
| Profile | ~400-800 | 0 (in MEMORY.md) | 400-800 |
| Status | ~200-400 | 0 (in MEMORY.md) | 200-400 |
| Duplicated instructions | ~200 | 0 | 200 |
| **Total savings** | | | **~800-1400 tokens** |

These tokens are not "free" — they move to MEMORY.md — but MEMORY.md is loaded regardless, so the net effect is deduplication.

---

## 10. Specification: Enhanced Skill Integration

### edi-core Skill Update

Add a new section to `edi-core/SKILL.md`:

```markdown
## Auto Memory Integration

EDI manages the project's MEMORY.md file. Follow these guidelines:

### Reading
- MEMORY.md is automatically loaded in your system prompt
- Use its content as primary project context
- For deeper knowledge, use recall_search

### Writing
- Do NOT write directly to MEMORY.md during normal work
- Memory updates happen through the /end workflow
- Exception: If you discover a critical insight (security issue, breaking change),
  note it for /end capture rather than writing immediately

### Referencing
- When MEMORY.md contains relevant patterns or pitfalls, reference them naturally
- Example: "Per our established pattern [from memory], using exponential backoff..."
- If memory content seems outdated, flag it at session end
```

### /end Command Update

Update the `/end` command template to include the MEMORY.md update step (as specified in Section 8).

---

## 11. Risk Analysis

### Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|
| MEMORY.md path changes in future Claude versions | EDI can't find/write memory | Low | Config override for memory path; detection fallback |
| Auto memory disabled by user | EDI writes to nowhere | Medium | Detect and fall back to current briefing behavior |
| MEMORY.md conflicts (user edits + EDI edits) | Content corruption | Medium | EDI reads before writing; preserves user-added sections |
| 200-line limit too restrictive | Can't fit all important content | Low | Topic files for overflow; strict line budgeting |
| Promotion noise (low-quality items in MEMORY.md) | Wasted tokens on noise | Medium | Conservative promotion criteria; user approval at /end |

### Backward Compatibility

EDI must work in three modes:

1. **Full auto memory** — Claude auto memory detected, EDI manages MEMORY.md
2. **EDI only** — No auto memory (older Claude Code version), current behavior
3. **Auto memory only** — Auto memory exists but EDI not managing it, don't overwrite

Detection logic:
```go
func DetermineMemoryMode(cfg *config.Config) MemoryMode {
    memDir, err := DetectAutoMemoryDir()
    if err != nil {
        return ModeEDIOnly // No auto memory directory
    }

    if cfg.Memory.Enabled == false {
        return ModeAutoMemoryPassive // Don't manage, don't overwrite
    }

    return ModeFullAlignment // EDI manages MEMORY.md
}
```

---

## 12. Success Criteria

### Functional

- [ ] `edi init` seeds MEMORY.md from `.edi/profile.md`
- [ ] `edi` (launch) updates MEMORY.md with current status and promoted RECALL items
- [ ] `/end` updates MEMORY.md with session insights and new captures
- [ ] Context file excludes profile/status when MEMORY.md is active (deduplication)
- [ ] RECALL items track promotion status
- [ ] Topic files generated for domains with 5+ items
- [ ] Graceful fallback when auto memory not available

### Performance

- [ ] MEMORY.md generation adds < 500ms to launch time
- [ ] MEMORY.md stays under 200 lines
- [ ] No increase in MCP latency (RECALL unchanged)

### Quality

- [ ] Only high-signal items promoted to MEMORY.md
- [ ] User has approval control over what enters MEMORY.md
- [ ] No data loss — all content still in RECALL, MEMORY.md is derived view

---

## Appendix A: File Changes Summary

### New Files
| File | Purpose |
|---|---|
| `edi/internal/memory/generator.go` | MEMORY.md content generation |
| `edi/internal/memory/topics.go` | Topic file management |
| `edi/internal/memory/sync.go` | RECALL-to-MEMORY.md sync logic |
| `edi/internal/memory/promote.go` | Promotion criteria and selection |
| `edi/internal/memory/detect.go` | Auto memory directory detection |
| `edi/internal/cli/memory.go` | `edi memory` CLI subcommand |
| `docs/architecture/auto-memory-alignment-spec.md` | This specification |

### Modified Files
| File | Change |
|---|---|
| `edi/internal/cli/launch.go` | Add memory generation step |
| `edi/internal/cli/init.go` | Seed MEMORY.md on init |
| `edi/internal/briefing/generator.go` | Skip profile/status when memory active |
| `edi/internal/launch/context.go` | Slimmer context when memory active |
| `edi/internal/config/schema.go` | Add `Memory` config section |
| `edi/internal/assets/commands/end.md` | Add MEMORY.md update step |
| `edi/internal/assets/skills/edi-core/SKILL.md` | Add auto memory guidelines |

### Unchanged Files
| Component | Reason |
|---|---|
| RECALL MCP server (v0 and Codex) | No protocol changes needed |
| Agent definitions | Memory awareness added via skill, not per-agent |
| Other skills (coding, testing, etc.) | No memory-specific behavior needed |
| Ralph loop | Operates with fresh context per iteration |
| Task system | No changes to task storage or sync |

---

## Appendix B: Decision Log

| Decision | Rationale |
|---|---|
| EDI co-manages MEMORY.md via merge (not overwrite) | Preserves Claude's auto-saved memories while maintaining structured EDI sections |
| Profile/status move to MEMORY.md | Eliminates token duplication between context file and auto memory |
| Conservative promotion criteria | Prevents noise; only proven-useful items enter MEMORY.md |
| Topic files for overflow | 200-line limit requires prioritization; depth goes to topic files |
| User approval at /end | Maintains human curation principle from EDI's design philosophy |
| Three operational modes | Backward compatibility with older Claude Code or disabled auto memory |

---

## Appendix C: Design Review Outcomes (February 9, 2026)

The following decisions were made during design review, superseding initial proposals where noted:

| # | Topic | Initial Proposal | Final Decision | Rationale |
|---|---|---|---|---|
| 1 | **MEMORY.md ownership** | EDI fully manages, Claude cannot write | EDI manages + "EDI Observations" section for Claude session notes | Gives Claude a voice (concurrence/dissent) without freestyle chaos; section is re-evaluated each session |
| 2 | **Profile/status deduplication** | Move profile/status from context to MEMORY.md only | Keep both — no deduplication | Simpler, less risk; MEMORY.md is additive (promoted RECALL items + topic index) |
| 3 | **Bloat prevention** | Promotion criteria only | Fixed slot budget (10/10/10) as primary mechanism | Predictable size cap; decay and consolidation deferred until needed |
| 4 | **Implementation scope** | 4 phases | Phases 1 + 3 only (generation + /end capture) | Core read/write loop provides maximum learning signal; Phase 2 unnecessary per #2; Phase 4 is polish |
| 5 | **Promotion at /end** | Apply scoring thresholds to all types | Decisions and failures always promoted (already human-approved); patterns need explicit user confirmation | Reduces friction for high-confidence items |

### Deferred Items

| Item | Deferred Until |
|---|---|
| Phase 2: Context deduplication | Reconsidered if token budget becomes a problem |
| Phase 4: `edi memory` CLI, promotion tracking columns | After real usage validates the architecture |
| Decay/eviction by time | After fixed slot budget proves insufficient |
| LLM-based consolidation of related items | After fragmentation is observed in practice |

---

## Appendix D: Merge-Based Pivot (February 19, 2026)

### Problem Discovered

Research into Claude Code's auto memory feature revealed a critical conflict with the original design:

1. **Claude Code writes to MEMORY.md automatically** during sessions — saving patterns, debugging insights, and decisions without being prompted
2. **Original design had EDI overwrite MEMORY.md** on every launch with freshly generated content
3. **Result**: EDI would destroy Claude's auto-saved memories on each session start

### Solution: Merge-Based Co-Ownership

Instead of overwriting, EDI now uses a **section-aware merge** approach:

| Section Type | Owner | On Launch |
|---|---|---|
| Project Quick Reference | EDI | Regenerated from `.edi/profile.md` |
| Current State | EDI | Regenerated from `.edi/status.md` |
| Key Patterns | EDI | Regenerated from RECALL promotions |
| Known Pitfalls | EDI | Regenerated from RECALL promotions |
| Key Decisions | EDI | Regenerated from RECALL promotions |
| Topic Index | EDI | Regenerated from memory dir listing |
| EDI Observations | Claude | **Preserved** across launches |
| Any other sections | Claude/User | **Preserved** across launches |

### Implementation

The merge works by:
1. Parsing existing MEMORY.md into sections (by `##` headers)
2. Identifying which sections are EDI-managed vs. preserved
3. Generating fresh EDI content
4. Appending preserved sections after EDI content
5. Enforcing the 195-line budget on the merged result

Key functions in `edi/internal/memory/generator.go`:
- `parseMemorySections()` — splits content by `##` headers
- `extractPreservedSections()` — filters to non-EDI sections
- `mergeWithExisting()` — combines EDI + preserved content
- `WriteMemoryFile()` — orchestrates read→generate→merge→write

### Design Decisions

| Decision | Rationale |
|---|---|
| Section-based parsing (not line-based) | Robust to content changes within sections |
| EDI sections identified by heading name | Simple, no markers needed; heading names are stable |
| Preserved sections appended after EDI sections | EDI controls top-of-file structure; Claude's additions go at bottom |
| EDI Observations always present | Placeholder added if not found in existing content |
| Line budget applied after merge | Ensures final output respects 200-line system prompt limit |
