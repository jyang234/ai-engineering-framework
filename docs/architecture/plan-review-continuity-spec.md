# Plan Review Continuity Spec

**Status:** Draft
**Date:** 2026-03-05
**Scope:** Changes to plan-review skill, /review-plan command, /ralph command, /end command

## Problem

The architect → review → execute pipeline has two continuity gaps:

1. **Same-session gap:** After plan-review approves a plan, switching to `/ralph` requires re-explaining all decisions. The approved plan lives in conversation context but is not structured for reuse.

2. **Cross-session gap:** If the architect session ends and `/ralph` runs in a new session, approved decisions are only available if they were captured to RECALL at `/end`. The `/end` workflow surfaces capture candidates, but has no way to know which decisions were already reviewed and approved — they look the same as any other flight recorder entry.

Both gaps stem from the same root cause: **plan-review produces a verdict but does not persist its output in a form that downstream consumers can reference.**

## Design Constraints

These are existing AEF conventions that this spec must respect:

1. **`recall_add` is reserved for `/end` curation** (edi-core line 326). No auto-writing to RECALL during normal work.
2. **`/memories/session-cache.md` is the session persistence layer.** Survives compaction, available to all tools in the current session.
3. **`flight_recorder_log` is fire-and-forget audit trail.** Written to SQLite immediately, not referenced after the call.
4. **`/end` surfaces capture candidates** from session activity. Items worth keeping are presented to the user for approval before `recall_add`.
5. **Ralph expects self-contained stories.** Cannot query RECALL at runtime. All context must be baked into PRD.json story descriptions.

## Changes

### Change A: Plan-Review Writes Approved Decisions to `/memories/`

**What:** When plan-review issues "Approved" or "Approved with Conditions", it writes a structured summary of the plan's key decisions to `/memories/session-cache.md`.

**Why:** This makes the approved decisions available to `/ralph` in the same session without re-explaining. It also survives compaction if the conversation grows long between review and PRD authoring.

**Where:** `edi/internal/assets/skills/plan-review/SKILL.md`, new step between Phase 5 (Structured Output) and Phase 6 (Post-Review).

**Spec:**

Add **Phase 5.5: Persist Approved Decisions** after the verdict is rendered:

```markdown
## Phase 5.5: Persist Approved Decisions

If the verdict is **Approved** or **Approved with Conditions**:

Write the following to `/memories/session-cache.md`:

~~~markdown
## Approved Plan: [plan topic]
**Verdict:** [Approved | Approved with Conditions]
**Date:** [current date]
**Conditions:** [if any, otherwise omit]

### Decisions
- [Decision 1]: [rationale]
- [Decision 2]: [rationale]

### Scope
- **In scope:** [what the plan covers]
- **Out of scope:** [explicit exclusions]

### Key Constraints
- [Constraint 1]
- [Constraint 2]

### Components Affected
- [component]: [what changes]
~~~

Keep this concise — one line per decision, one line per scope item. This is working memory, not documentation.

If the verdict is **Revise**, do not write to `/memories/`. The plan is not yet approved.
```

**Also update the Phase 6 flight_recorder_log call** to include a `plan_approved` flag in metadata:

```markdown
## Phase 6: Post-Review

Log findings to the flight recorder:

~~~
flight_recorder_log({
  type: "observation",
  content: "Plan review: [Approved|Approved with Conditions|Revise] — [1-line summary]",
  metadata: {
    critical_risks: N,
    high_risks: N,
    yagni_violations: N,
    verdict: "[verdict]",
    plan_approved: true|false,
    plan_topic: "[brief topic]",
    conditions: ["condition 1", "condition 2"]
  }
})
~~~
```

The `plan_approved` and `plan_topic` fields allow `/end` to identify reviewed plans when surfacing capture candidates (see Change C).

### Change B: Named Review Gates

**What:** Rename plan-review's Phase 2, 3, and 4 from numbered phases to named gates. Update the verdict format to reference gate names.

**Why:** "Failed Complexity Gate" is more actionable than "Phase 3 concerns." When re-submitting a revised plan, both the human and the agent can reference specific gates by name.

**Where:** `edi/internal/assets/skills/plan-review/SKILL.md`, Phases 2-5.

**Spec:**

Rename:
- Phase 2: Regression Risk Assessment → **Regression Gate**
- Phase 3: Complexity Assessment → **Complexity Gate**
- Phase 4: Over-Engineering Detection (YAGNI) → **YAGNI Gate**

Phase 1 (RECALL Context Loading) and Phase 5 (Structured Output) keep their names — they are not gates.

Update the Phase 5 Structured Output format to use gate names:

```markdown
## Phase 5: Structured Output

Present findings in this format:

### Regression Gate

**Status:** Pass | Fail | Pass with Conditions

**Critical/High risks:**
- [Risk]: [What could go wrong] → [Suggested mitigation]

**Medium risks:**
- [Risk]: [What could go wrong] → [Suggested mitigation]

### Complexity Gate

**Status:** Pass | Fail | Pass with Conditions

| Item | Classification | Notes |
|------|---------------|-------|
| [change] | Justified / Questioned | [why] |

### YAGNI Gate

**Status:** Pass | Fail | Pass with Conditions

- [Item]: [Why it appears to be over-engineering]

### Missing Elements

- [ ] Test strategy for affected areas
- [ ] Rollback plan
- [ ] Migration path (if applicable)
- [ ] Performance impact assessment (if applicable)

### Verdict

Derived from gate statuses:
- **Approved** — All gates pass. Proceed to implementation.
- **Approved with Conditions** — All gates pass, but conditions must be addressed before or during implementation. [List conditions and which gate they came from.]
- **Revise** — One or more gates failed. [List failed gates with specific concerns.]

Example: "Revise — Complexity Gate failed (novel event sourcing pattern introduced without justification), YAGNI Gate failed (3 new config keys with no current consumer)."
```

### Change C: `/end` Surfaces Approved Plans as Priority Capture Candidates

**What:** The `/end` workflow checks flight recorder for entries with `plan_approved: true` and surfaces them as high-priority capture candidates with pre-formatted content.

**Why:** Approved architectural plans are the highest-signal decisions in a session. They have already been reviewed and vetted. The `/end` workflow should make it easy to promote them to RECALL rather than requiring the user to manually identify them from a list.

**Where:** `edi/internal/assets/commands/end.md`, Step 3 (Identify capture candidates).

**Spec:**

Update Step 3 to prioritize approved plans:

```markdown
3. **Identify** capture candidates — things worth saving to RECALL:

   **Priority candidates** (already reviewed and approved this session):
   - Check `/memories/session-cache.md` for "Approved Plan:" sections
   - These have already passed plan-review and should be presented first

   **Other candidates:**
   - New patterns discovered
   - Failures encountered and fixed
   - Important decisions with rationale (not already covered by approved plans)
```

Update Step 4 presentation to distinguish priority candidates:

```markdown
4. **Present** capture candidates to user:
   ~~~
   I identified these items worth capturing to RECALL:

   Reviewed and approved this session:
   [1] Decision: [approved plan topic] — [1-line summary of key decisions]

   Other candidates:
   [2] Pattern: [description]
   [3] Failure: [description]

   Capture to RECALL? [A]ll / [1-3] Select / [S]kip
   ~~~
```

The approved plan capture uses the existing decision format from Step 5 of `/end`:

```
recall_add({
  type: "decision",
  title: "[plan topic] — architectural decisions",
  content: "## Context\n[problem being solved]\n\n## Decision\n[key decisions from the approved plan]\n\n## Alternatives Considered\n[from the plan review]\n\n## Consequences\n[trade-offs acknowledged]\n\n## Review\nApproved by plan-review on [date]. Conditions: [if any].",
  tags: ["plan-review", "approved", "[domain tags]"]
})
```

### Change D: `/ralph` Loads Approved Plan Context

**What:** `/ralph`'s Phase 1 (Discovery) checks `/memories/session-cache.md` for an approved plan before starting the interview. If found, it pre-populates context and skips redundant discovery questions.

**Why:** In same-session workflows (architect → review → ralph), the user should not have to re-explain decisions that were just approved. In cross-session workflows, `/ralph` falls back to RECALL queries for prior decisions.

**Where:** `edi/internal/assets/commands/ralph.md`, Phase 1.

**Spec:**

Add **Phase 0: Load Prior Context** before the existing Phase 1:

```markdown
## Workflow

### Phase 0: Load Prior Context

Before starting discovery, check for existing approved plans:

1. **Same-session context:** Read `/memories/session-cache.md` for "Approved Plan:" sections
2. **Cross-session context:** Query RECALL for recent decisions in the relevant domain:
   ~~~
   recall_search({query: "[project/feature area]", types: ["decision"]})
   ~~~
   Apply retrieval-judge to filter results.

If an approved plan is found:

~~~
I found an approved plan for [topic] from [this session / a prior session].

Decisions already made:
- [Decision 1]
- [Decision 2]

Scope: [in-scope summary]
Constraints: [key constraints]

I will use these as the foundation for the PRD. Let me confirm:
1. Is this still the plan we are executing?
2. Any changes since the review?
~~~

If confirmed, skip Phase 1 discovery questions that are already answered by the plan. Proceed to Phase 2 (Story Breakdown) with the plan decisions pre-loaded.

If no approved plan is found, proceed to Phase 1 as normal.
```

Update Phase 3 (Front-Load Decisions) to reference the approved plan:

```markdown
### Phase 3: Front-Load Decisions

For each story, ensure the description contains ALL context Ralph will need:

- **Decisions from approved plan** — if Phase 0 loaded an approved plan, every relevant decision must appear in the story description. Do not assume Ralph has access to the plan.
- Architecture decisions that affect implementation
- [... rest of existing Phase 3 unchanged ...]
```

## Files Changed

| File | Change |
|------|--------|
| `edi/internal/assets/skills/plan-review/SKILL.md` | Add Phase 5.5 (persist approved decisions to `/memories/`), rename Phases 2-4 to named gates, update Phase 5 output format, update Phase 6 flight_recorder metadata |
| `edi/internal/assets/commands/review-plan.md` | No changes needed — it delegates to the plan-review skill |
| `edi/internal/assets/commands/ralph.md` | Add Phase 0 (load prior context), update Phase 3 reference |
| `edi/internal/assets/commands/end.md` | Update Step 3-4 to prioritize approved plan candidates |

## What This Does NOT Change

- **Architect agent** (`architect.md`) — No format changes to architect output. The architect continues to produce ADR-style prose. Plan-review evaluates whatever it receives.
- **RECALL storage** — No schema changes. Approved plans are stored as standard `decision` type items.
- **`recall_add` convention** — Still reserved for `/end` curation. Plan-review writes to `/memories/`, not RECALL.
- **Refactoring-planning skill** — Unchanged. Its YAML spec format remains domain-specific for refactoring.
- **Reviewer agent** — Unchanged. It already delegates to plan-review skill.

## Interaction Diagram

```
Same-session flow:

  /plan (architect)
    → Produces plan (ADR prose, any format)
    → flight_recorder_log(type="decision", ...)

  /review-plan (reviewer + plan-review skill)
    → RECALL Context Loading
    → Regression Gate
    → Complexity Gate
    → YAGNI Gate
    → Verdict: Approved
    → Writes "Approved Plan: ..." to /memories/session-cache.md    ← NEW
    → flight_recorder_log(..., plan_approved: true)                ← UPDATED

  /ralph (PRD authoring)
    → Phase 0: Reads /memories/session-cache.md                   ← NEW
    → Finds approved plan, confirms with user
    → Skips redundant discovery
    → Phase 2: Story Breakdown (with plan decisions pre-loaded)
    → Phase 3: Bakes all decisions into story descriptions
    → PRD.json

  /end
    → Surfaces approved plan as priority capture candidate         ← NEW
    → User approves → recall_add(type="decision", ...)
    → Available to future sessions via recall_search


Cross-session flow:

  Session 1: /plan → /review-plan → /end (captures approved plan to RECALL)

  Session 2: /ralph
    → Phase 0: /memories/ empty (new session)
    → Phase 0: recall_search({types: ["decision"]})               ← NEW
    → Finds approved plan from Session 1
    → Confirms with user, proceeds to story breakdown
```

## Success Criteria

1. In same-session flow: `/ralph` does not re-ask questions whose answers are in the approved plan
2. In cross-session flow: `/ralph` finds and uses approved plans captured to RECALL at prior `/end`
3. Plan-review verdicts reference gates by name ("failed Complexity Gate")
4. `/end` presents approved plans as first-priority capture candidates
5. No changes to the `recall_add` convention — curation still happens at `/end`
6. No changes to architect output format — plan-review remains format-agnostic
