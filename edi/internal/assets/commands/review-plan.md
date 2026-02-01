---
name: review-plan
description: Review an architectural plan for regression risk and unnecessary complexity
---

# Review Architectural Plan

Reviewing plan for regression risk, complexity, and over-engineering.

## Actions

1. Acknowledge: "Reviewing plan for regression risk, complexity, and over-engineering."
2. Read the plan file. Check Claude Code's plan mode file first, or ask the user to specify which plan to review.
3. Query RECALL for failures in affected areas:
   ```
   recall_search({query: "[affected domain areas]", types: ["failure", "pattern"]})
   ```
4. Query RECALL for relevant decisions and constraints:
   ```
   recall_search({query: "[affected domain areas]", types: ["decision", "context"]})
   ```
5. Apply retrieval-judge to filter results
6. Execute the plan-review skill framework against the plan:
   - Phase 2: Regression Risk Assessment
   - Phase 3: Complexity Assessment
   - Phase 4: Over-Engineering Detection (YAGNI)
   - Phase 5: Structured Output with verdict
7. Log findings to flight_recorder_log:
   ```
   flight_recorder_log({
     type: "observation",
     content: "Plan review: [verdict] — [summary]",
     metadata: {critical_risks: N, high_risks: N, yagni_violations: N, verdict: "[verdict]"}
   })
   ```
8. Present structured assessment with approval status

## Response

```
Reviewing plan for regression risk, complexity, and over-engineering.

[RECALL context loaded — X/Y results kept]

[Structured assessment from plan-review skill]

[Verdict: Approved / Approved with Conditions / Revise]
```
