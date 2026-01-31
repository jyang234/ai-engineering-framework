# Ralph Wiggum Loop Specification

> **Implementation Status (January 31, 2026):** Implemented as described.

**Status:** Draft
**Version:** 1.0
**Date:** January 31, 2026

---

## Overview

Ralph Wiggum loops are an autonomous execution pattern for well-defined coding tasks. Each iteration starts with a fresh context window, reads the current task from a spec file, implements it, and exits. State persists in files and git, not in the LLM's memory.

### Core Properties

| Property | Description |
|----------|-------------|
| **Fresh context** | Each iteration starts clean—no accumulated cruft |
| **Focused execution** | One task per iteration, full attention |
| **External state** | Progress tracked in PRD.json and git, not context |
| **Human escalation** | Safety valve when stuck or spec is wrong |
| **Autonomous** | Runs unattended until completion or escalation |

### When to Use Ralph

**Good fit:**
- Batch of independent, well-specified tasks
- Executing a reviewed and complete spec
- Grunt work where focus > creativity
- Tasks with clear acceptance criteria and verification

**Poor fit:**
- Exploratory or debugging work
- Architecture and design decisions
- Multi-file refactors requiring consistent vision
- Security-sensitive code requiring threat model awareness
- Interconnected tasks where decisions compound

**Rule of thumb:** If you can write complete acceptance criteria before starting and tasks don't share in-flight decisions, Ralph is appropriate.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Ralph Loop                                │
│                                                                  │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐  │
│   │  Read   │────▶│  Build  │────▶│  Run    │────▶│ Analyze │  │
│   │  Task   │     │ Prompt  │     │ Claude  │     │ Output  │  │
│   └─────────┘     └─────────┘     └─────────┘     └─────────┘  │
│        │                                               │        │
│        │              ┌─────────────────┐              │        │
│        │              │                 │              │        │
│        │              ▼                 │              ▼        │
│        │      ┌─────────────┐    ┌─────────────┐  ┌────────┐   │
│        │      │  Escalate   │    │   Update    │  │  Next  │   │
│        │      │  to Human   │    │  PRD + Git  │  │  Task  │   │
│        │      └─────────────┘    └─────────────┘  └────────┘   │
│        │              │                                 │       │
│        │              ▼                                 │       │
│        │      ┌─────────────┐                          │       │
│        │      │   Human     │──────────────────────────┘       │
│        │      │   Input     │                                   │
│        └──────┴─────────────┴───────────────────────────────────│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

External State:
├── PRD.json          Task backlog with completion status
├── PROMPT.md         Execution instructions
├── .ralph/           Working directory (gitignored)
└── Git history       Code changes, commits
```

---

## File Format

### PRD.json

```json
{
  "project": "project-name",
  "description": "Brief project description",
  "userStories": [
    {
      "id": "US-001",
      "title": "Short title",
      "description": "Full description of what to implement",
      "criteria": [
        "First acceptance criterion",
        "Second acceptance criterion"
      ],
      "passes": false
    },
    {
      "id": "US-002",
      "title": "Dependent task",
      "description": "This task depends on US-001",
      "criteria": [
        "Acceptance criterion"
      ],
      "passes": false,
      "depends_on": ["US-001"]
    }
  ]
}
```

**Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique task identifier |
| `title` | Yes | Short descriptive title |
| `description` | Yes | Full implementation description |
| `criteria` | Yes | List of acceptance criteria (verifiable) |
| `passes` | Yes | Completion status (false → true when done) |
| `depends_on` | No | List of task IDs that must complete first |
| `skipped` | No | Set to true if human skips task |

### PROMPT.md

Instructions provided to Claude each iteration. Should include:

1. **Execution guidelines** — How to approach the task
2. **Escalation protocol** — When and how to escalate
3. **Completion format** — How to signal task done

See appendix for template.

---

## Escalation Protocol

### When to Escalate

**STUCK** — Cannot make progress:
- Same error 3+ times despite different approaches
- Blocked by external factors (missing credentials, unavailable services)
- Genuinely doesn't know how to proceed

**DEVIATION** — Can proceed but shouldn't without approval:
- Spec appears wrong, incomplete, or contradictory
- Task scope significantly larger than criteria suggest
- Need to modify code outside task's stated scope
- Security or compliance concern discovered
- Architectural decision required not covered in spec

### Escalation Format

Claude outputs this block and stops:

```xml
<escalate type="stuck|deviation">
<summary>One-line description</summary>
<context>
What was being attempted.
What happened or was discovered.
What was already tried (if stuck).
</context>
<options>
1. First possible path forward
2. Second possible path forward
3. Third option if applicable
</options>
<question>Specific question for the human</question>
</escalate>
```

### Human Response Options

When escalation detected, human can:

| Option | Effect |
|--------|--------|
| Select numbered option | Guidance injected: "Proceed with option N" |
| Custom guidance | Free-form text injected into next prompt |
| Skip task | Task marked skipped, loop continues to next |
| Retry | Re-run iteration without additional guidance |
| Abort | Exit loop entirely |

### Auto-Escalation

Loop script detects repeated failures and auto-escalates:

```
Iteration N:   Error: ConnectionRefused
Iteration N+1: Error: ConnectionRefused  
Iteration N+2: Error: ConnectionRefused
→ Auto-escalate: "Same error 3 times, how to proceed?"
```

Threshold configurable (default: 3 consecutive identical errors).

---

## Loop Mechanics

### Task Selection

1. Filter tasks where `passes == false` and `skipped != true`
2. Filter tasks where all `depends_on` tasks have `passes == true`
3. Select first remaining task

### Completion Detection

Task marked complete when output contains clear completion signal:
- "Task [ID] complete"
- "Task [ID] done"
- Similar unambiguous patterns

**All tasks complete** when output contains:
```
<promise>DONE</promise>
```

### Progress Tracking

After each iteration:
1. If task complete: Update `passes: true` in PRD.json
2. If task skipped: Update `skipped: true` in PRD.json  
3. Commit changes to git with descriptive message

### Iteration Limits

Default maximum: 50 iterations

If limit reached without completion:
- Log warning
- Exit with non-zero status
- Human investigates

---

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_ITERATIONS` | 50 | Maximum loop iterations |
| `STUCK_THRESHOLD` | 3 | Consecutive errors before auto-escalate |

---

## Integration with EDI

Ralph is an optional execution mode, not a replacement for the coding subagent.

### Invocation

```bash
# Standalone
./ralph.sh

# Via EDI (future)
edi ralph PRD.json
```

### Relationship to RECALL

RECALL is **not** used during Ralph execution. Context retrieval belongs in the planning phase:

```
Planning (uses RECALL)     →  PRD.json  →  Execution (Ralph, no RECALL)
```

If a task needs dynamic context retrieval, it's not ready for Ralph. Fix the spec.

### Relationship to Sandbox

Sandbox (when implemented) could verify task completion:

```
Task complete? → Run Sandbox experiment → Passes? → Mark complete
                                        → Fails?  → Retry or escalate
```

This is future work, not current scope.

---

## Appendix: PROMPT.md Template

```markdown
# Ralph Loop Instructions

You are executing one task from a product backlog. Work autonomously but escalate when appropriate.

## Execution

- Implement the task to meet all acceptance criteria
- Run tests to verify correctness
- Keep changes focused—don't expand scope

## Escalation

You MUST escalate in these situations:

### STUCK — Cannot make progress
- Same error 3+ times despite different approaches
- Blocked by external factors
- Genuinely don't know how to proceed

### DEVIATION — Can proceed but shouldn't without approval
- Spec appears wrong, incomplete, or contradictory
- Task scope larger than criteria suggest
- Need to modify code outside task's stated scope
- Security or compliance concern discovered

### Format

<escalate type="stuck|deviation">
<summary>One-line description</summary>
<context>
What you were doing.
What happened.
What you tried (if stuck).
</context>
<options>
1. First option
2. Second option
</options>
<question>Your question</question>
</escalate>

After escalating: STOP. Do not continue. Do not output <promise>DONE</promise>.

## Completion

When task complete and tests pass:

1. State clearly: "Task [ID] complete"

2. If ALL tasks done, output: <promise>DONE</promise>

## Guidelines

- Stay focused on THIS task only
- Escalate early rather than spin
- Be explicit about completion
```

---

## Appendix: Example Session

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 Ralph Loop Starting
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:00:01] Progress: 0/5 complete (5 remaining)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 Iteration 1: US-001
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:00:01] Running Claude...
... Claude implements task ...
[10:01:23] ✓ Task US-001 complete

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 Iteration 2: US-002
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:01:25] Running Claude...
... Claude encounters issue ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚨 ESCALATION REQUIRED
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

<escalate type="deviation">
<summary>Spec requires deprecated API endpoint</summary>
<context>
US-002 specifies using /api/v1/users but codebase uses /api/v2/users.
The v1 endpoint is marked deprecated in the OpenAPI spec.
</context>
<options>
1. Use v2 endpoint instead (update acceptance criteria)
2. Use v1 anyway (technical debt)
3. Skip task pending spec clarification
</options>
<question>Which API version should I target?</question>
</escalate>

Options: [1-9] Select option  [c] Custom  [s] Skip  [r] Retry  [a] Abort
Choice: 1

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 Iteration 3: US-002 (with guidance)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:02:45] Running Claude...
... Claude implements with v2 API ...
[10:03:52] ✓ Task US-002 complete

... continues ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ All Tasks Complete!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[10:15:33] Final: 5/5 complete (0 remaining)
```

---

## References

- [Ralph Wiggum technique](https://www.aihero.dev/tips-for-ai-coding-with-ralph-wiggum) — Matt Pocock
- ADR: Ralph Wiggum Loop Integration with EDI
- EDI Subagent Specification
