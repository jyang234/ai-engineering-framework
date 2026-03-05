---
name: plan-review
description: Review architectural plans for regression risk, unnecessary complexity, and over-engineering
---

# Plan Review

Structured framework for reviewing architectural plans before implementation begins. Catches regressions, unnecessary complexity, and over-engineering at design time — before code is written.

## RECALL Context Loading

Before reviewing the plan, load relevant context:

```
recall_search({query: "[affected domain areas]", types: ["failure", "pattern"]})
recall_search({query: "[affected domain areas]", types: ["decision", "context"]})
```

Apply retrieval-judge to filter results. Cross-reference plan changes against past failures and existing decisions.

After filtering, write relevant findings to `/memories/session-cache.md` so they survive compaction:
```
"Plan review for [area]: found [N] relevant RECALL items — [1-line summary of key findings]"
```

## Regression Gate

Evaluate each change area for regression risk:

| Factor | Question |
|--------|----------|
| **Blast radius** | What depends on this? How many callers/consumers are affected? |
| **Past failures** | Has RECALL surfaced failures in this area? |
| **Test coverage** | Are the affected areas well-tested? What gaps exist? |
| **Rollback strategy** | Does the plan describe how to undo changes if they fail? |

Categorize each change:

- **Data layer** — Schema changes, migrations, storage format changes
- **API/Interfaces** — Public API changes, interface modifications, protocol changes
- **Dependencies** — New or upgraded dependencies, version changes
- **Configuration** — New config keys, environment variables, feature flags
- **Business logic** — Rule changes, workflow modifications, state machine changes

Flag any change area with past failures and no explicit mitigation in the plan.

## Complexity Gate

Flag these red patterns:

- **New abstraction for single use case** — Adding a layer (interface, wrapper, factory) that has exactly one implementation and no stated plan for a second
- **Novel patterns when existing ones work** — Introducing a new approach (e.g., event sourcing) when the project already has a working pattern for the same problem
- **Premature optimization** — Performance-motivated changes without profiling data or benchmarks showing a problem
- **Multiple new technologies** — Adding new languages, frameworks, or infrastructure when the existing stack can handle the requirement

For each flagged item, classify as:
- **Justified** — The plan explains why the complexity is necessary and the simpler alternative won't work
- **Questioned** — The complexity may be warranted but the plan doesn't justify it

## YAGNI Gate

Check for these signals:

- **Future-driven design** — Is a feature or abstraction motivated by "we might need this later" rather than a current requirement?
- **Missing scope boundary** — Does the plan state what is explicitly OUT of scope?
- **File ratio** — Count new files vs modified files. A high ratio of new files suggests new abstractions rather than extending existing code.
- **Dependency count** — How many new dependencies does the plan introduce? Each is a maintenance burden.
- **Configuration surface** — Does the plan add configurability that no current user needs?

## Structured Output

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

## Persist Approved Decisions

If the verdict is **Approved** or **Approved with Conditions**:

Write the following to `/memories/session-cache.md`:

```markdown
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
```

Keep this concise — one line per decision, one line per scope item. This is working memory, not documentation.

If the verdict is **Revise**, do not write to `/memories/`. The plan is not yet approved.

## Post-Review

Log findings to the flight recorder:

```
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
```
