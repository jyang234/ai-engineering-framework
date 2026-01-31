# ADR: Ralph Wiggum Loop Integration with EDI

> **Implementation Status (January 31, 2026):** Implemented as described.

**Status:** Accepted
**Date:** January 31, 2026
**Deciders:** John Yang
**Context:** AEF/EDI Architecture

---

## Context

Ralph Wiggum loops are an emerging pattern for autonomous AI coding that solves context rot by embracing fresh starts. Each iteration begins with a clean context window, reads state from disk (PRD, git history), completes one task, and exits.

We evaluated whether and how to integrate Ralph loops with EDI, and specifically whether RECALL/Codex should be part of the execution loop.

## Decision

### 1. Ralph is a separate execution mode, not a replacement for the coding subagent

**Rationale:**

Ralph trades context continuity for focus. This tradeoff is beneficial for independent, well-specified tasks but harmful for interconnected, exploratory, or security-sensitive work.

| Work Type | Better Fit |
|-----------|------------|
| Batch of independent implementation tasks | Ralph |
| Architecture and design | Continuous session |
| Debugging complex issues | Continuous session |
| Multi-file refactors needing consistent vision | Continuous session |
| Security-sensitive code | Continuous session |
| Executing a reviewed, complete spec | Ralph |

The human should explicitly choose Ralph mode when appropriate, not have EDI route automatically. Automatic routing adds complexity for unclear benefit and will mishandle edge cases.

### 2. RECALL is for the planning phase, not the Ralph execution loop

**Rationale:**

We initially designed RECALL integration into the Ralph execution loop (Claude would query for context each iteration). This was architecturally incorrect for three reasons:

1. **Technical:** `claude -p` (pipe mode) doesn't support MCP tools. Claude would see instructions to call RECALL but couldn't execute them.

2. **Conceptual:** Ralph's value is focused execution of well-defined specs. If the spec needs RECALL queries to be complete, the spec isn't ready for Ralph execution. RECALL belongs in the planning phase when the PRD is written.

3. **Practical:** Pre-baking context into the spec is more reliable than dynamic retrieval. The human can curate what's relevant during planning rather than hoping retrieval finds the right context at execution time.

**Correct flow:**

```
Planning Phase (interactive, uses RECALL)
    │
    ├── Human + Claude query RECALL for patterns, decisions, failures
    ├── Design informed by organizational knowledge  
    ├── Write PRD.json with complete specs
    └── All relevant context baked INTO the spec
    │
    ▼
Execution Phase (Ralph loop, no RECALL)
    │
    ├── Read task from PRD.json
    ├── Implement exactly what spec says
    ├── Escalate if spec is wrong/incomplete
    └── Complete task, move to next
    │
    ▼
Capture Phase (optional, post-execution)
    │
    ├── Human reviews completed work
    └── Captures new patterns/failures to RECALL for future planning
```

### 3. Ralph requires human escalation protocol

**Rationale:**

Autonomous loops need a safety valve. Ralph should escalate to human when:

- **Stuck:** Same error repeatedly, blocked by external factors, doesn't know how to proceed
- **Deviation:** Spec appears wrong, scope larger than expected, would require out-of-scope changes, security/compliance concern

Without escalation, Ralph either spins indefinitely or makes autonomous decisions that should require approval.

## Consequences

### Positive

- Clear separation of concerns (planning vs execution)
- Human retains control over when to use autonomous execution
- No false claims about RECALL integration that doesn't work
- Simpler Ralph implementation (no MCP dependency)
- RECALL's value preserved for where it matters (planning)

### Negative

- Two modes to understand (Ralph vs continuous)
- Human must judge task suitability for Ralph
- No automatic organizational learning from Ralph execution (capture is manual)

### Neutral

- Ralph remains optional—teams can ignore it if they prefer continuous sessions
- Existing EDI/Codex architecture unchanged

## Alternatives Considered

### Alternative 1: Auto-route tasks to Ralph or continuous

Rejected. Heuristics for routing are fragile. Edge cases will be misrouted. Human judgment is more reliable for this decision.

### Alternative 2: Pre-fetch RECALL context in loop script

Considered but rejected. If context needs to be fetched, the spec isn't complete. Fix the spec in planning, don't patch it in execution.

### Alternative 3: Run Ralph with full MCP (Claude Code instead of claude -p)

Technically possible but conceptually wrong. Ralph's value is minimal context. Adding RECALL queries expands context and defeats the purpose.

## Implementation Notes

Ralph loop implementation should include:

1. **Loop mechanics:** Iterate through PRD.json tasks, detect completion, commit progress
2. **Human escalation:** Detect escalation blocks in output, prompt human, inject guidance
3. **Auto-escalation:** Detect repeated failures, escalate automatically
4. **No RECALL dependency:** Works standalone, MCP optional

Invocation pattern:
```bash
# Regular coding (continuous context)
edi coder "implement the auth system"

# Ralph mode (explicit)
edi ralph PRD.json

# Or standalone
./ralph.sh
```

## References

- [Ralph Wiggum technique](https://www.aihero.dev/tips-for-ai-coding-with-ralph-wiggum) - Matt Pocock
- AEF Architecture Specification v0.5
- EDI Subagent Specification
