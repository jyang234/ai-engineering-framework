---
name: plan
aliases:
  - architect
  - design
description: Switch to architect mode for system design
---

# Switch to Architect Mode

Transitioning to architect mode for system design work.

## Actions

1. Acknowledge the mode switch
2. Query RECALL for relevant architectural context:
   ```
   recall_search({query: "[current project/domain]", types: ["decision", "pattern"]})
   ```
3. Log the agent switch:
   ```
   flight_recorder_log({
     type: "agent_switch",
     content: "Switching to architect mode",
     from_agent: "[previous]",
     to_agent: "architect"
   })
   ```

## Architect Mode Focus

- System-level thinking
- Trade-offs and implications
- ADR documentation
- Task breakdown
- Long-term maintainability

## Response

```
Switching to architect mode.

[If RECALL has relevant context, summarize it]

How can I help with system design?
```
