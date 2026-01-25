# AEF Architecture v0.5 Changelog

**Date**: January 23, 2026  
**Trigger**: Claude Code Tasks announcement (Jan 22, 2026)

---

## Executive Summary

Claude Code's Tasks feature (announced Jan 22, 2026) provides native support for:
- Task dependencies
- Cross-session collaboration via `CLAUDE_CODE_TASK_LIST_ID`
- Broadcast updates to all sessions on same Task List
- File-based persistence in `~/.claude/tasks/`

This significantly overlaps with AEF's planned Claude Harness functionality. v0.5 revises the architecture to **build on Claude Code primitives** rather than replace them.

---

## Architectural Shift

### Before (v0.4): AEF Builds Everything

```
AEF Harness
├── Subagent orchestration
├── Task decomposition
├── Cross-session state
├── Working memory
├── Persistence layer
└── All coordination logic
```

### After (v0.5): AEF as Value Layer

```
Claude Code (Foundation)
├── Tasks (coordination)
├── Skills (behaviors)
├── Subagents (delegation)
└── Sessions (persistence)
        │
        ▼
AEF Value Layer (Built on top)
├── Codex (knowledge retrieval)
├── Evaluation + Self-Correct (quality gates)
├── Contribution Manager (knowledge loop)
├── Audit Trail (decision history)
└── Org DNA (as Skills format)
```

---

## Components Changed

### Deprecated/Absorbed

| Component | v0.4 Status | v0.5 Status | Replaced By |
|-----------|-------------|-------------|-------------|
| Claude Harness | ✅ Specified | ❌ Absorbed | Claude Code Tasks + Subagents |
| Subagent Model | ✅ Specified | ❌ Absorbed | Claude Code native subagents |
| Working Memory | ✅ Specified | ❌ Absorbed | Claude Code Tasks + Sessions |

### New/Revised

| Component | v0.5 Status | Purpose |
|-----------|-------------|---------|
| Claude Code Integration Layer | ✅ NEW | Hooks into Tasks lifecycle |
| Org DNA as Skills | ✅ REVISED | Portable SKILL.md format |
| Role System | ✅ Preserved | Implemented via Skills |

### Unchanged (Unique AEF Value)

| Component | Rationale |
|-----------|-----------|
| Codex | Claude Code has no knowledge base |
| CAL | Context assembly still needed |
| Evaluation + Self-Correct | No quality gates in Claude Code |
| Contribution Manager | Artifacts don't auto-flow to knowledge |
| Audit Trail | Tasks store state, not reasoning |

---

## Integration Model

AEF injects value at Claude Code lifecycle events:

```yaml
hooks:
  PreTaskStart:
    - query_codex (inject relevant context)
    - load_relevant_skills (from org_dna)
    
  PostTaskComplete:
    - run_evaluation (rubrics, self-correct on fail)
    - prompt_contribution (if significant)
    
  OnTaskFail:
    - capture_audit_trail (context, error, reasoning)
    - trigger_self_correct (max 3 attempts)
```

---

## Decision Log Additions (v0.5)

| Decision | Rationale |
|----------|-----------|
| Build on Claude Code Tasks, not replace | Native dependencies, cross-session broadcast |
| Org DNA implemented as Claude Code Skills | Portable format, ecosystem compatibility |
| Absorb Claude Harness into primitives | Subagents, Tasks, Sessions provide orchestration |
| AEF as hook layer, not wrapper | Injection at lifecycle points |
| Roles preserved via Skills | Semantic layer over behavioral Skills |
| Audit Trail remains | Tasks are state, not reasoning |
| Codex standalone | Knowledge retrieval is unique value |
| Use CLAUDE_CODE_TASK_LIST_ID | Native cross-session state |

---

## Implementation Priority (Revised)

| Priority | Component | Rationale |
|----------|-----------|-----------|
| **P0** | Codex | No Claude Code equivalent |
| **P0** | Evaluation + Self-Correct | No quality gates |
| **P0** | Contribution Manager | Close knowledge loop |
| **P1** | Audit Trail | Capture reasoning |
| **P1** | CAL | Context assembly |
| **P1** | Org DNA as Skills | Behavioral consistency |
| **P2** | Role System | Semantic layer |
| ~~Defer~~ | ~~Orchestration~~ | Use Claude Code Tasks |
| ~~Defer~~ | ~~Working Memory~~ | Use Tasks + Sessions |

---

## Migration Notes

For users referencing v0.4 Claude Harness specification:

1. **Subagent Definitions** → Convert to Claude Code Skills format
2. **Role Configs** → Convert to role-specific Skills
3. **Working Memory** → Use `CLAUDE_CODE_TASK_LIST_ID` for project scoping
4. **Persistence Layer** → Audit Trail remains; Working Memory absorbed
5. **CAL Integration** → Unchanged; hooks into PreTaskStart

**Example Usage (v0.5):**

```bash
export CLAUDE_CODE_TASK_LIST_ID=my-project
export AEF_CODEX_PROJECT=my-project
export AEF_ROLE=architect

claude --skills ~/.claude/skills/org-dna,~/.claude/skills/roles/architect
```

---

## Files Updated

| File | Change |
|------|--------|
| `aef-architecture-specification-v0.5.md` | Full architecture revision |
| `claude-harness-deep-dive.md` | Historical reference (v0.4 design) |

---

## What This Means for AEF

**Scope Shrinks**: Don't rebuild what Claude Code provides  
**Value Sharpens**: Focus on knowledge retrieval, quality assurance, organizational learning  
**Integration Path**: Build on Claude Code hooks and Skills format  

AEF becomes the **knowledge and quality layer** on top of Claude Code's agentic capabilities.
