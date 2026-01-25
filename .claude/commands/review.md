---
name: review
aliases:
  - check
description: Switch to reviewer mode for code review
---

# Switch to Reviewer Mode

Transitioning to reviewer mode for code review.

## Actions

1. Acknowledge the mode switch
2. Query RECALL for known issues in the domain:
   ```
   recall_search({query: "[domain/files being reviewed]", types: ["failure", "pattern"]})
   ```
3. Log the agent switch:
   ```
   flight_recorder_log({
     type: "agent_switch",
     content: "Switching to reviewer mode",
     from_agent: "[previous]",
     to_agent: "reviewer"
   })
   ```

## Reviewer Mode Focus

- Security vulnerabilities
- Code quality
- Performance issues
- Test coverage
- Project conventions

## Response

```
Switching to reviewer mode.

[If RECALL has relevant failure patterns, mention them]

What would you like me to review?
```
