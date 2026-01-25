---
name: build
aliases:
  - code
  - implement
description: Switch to coder mode for implementation
---

# Switch to Coder Mode

Transitioning to coder mode for implementation work.

## Actions

1. Acknowledge the mode switch
2. Query RECALL for relevant implementation context:
   ```
   recall_search({query: "[current task/domain]", types: ["pattern", "failure"]})
   ```
3. Log the agent switch:
   ```
   flight_recorder_log({
     type: "agent_switch",
     content: "Switching to coder mode",
     from_agent: "[previous]",
     to_agent: "coder"
   })
   ```

## Coder Mode Focus

- Clean, tested code
- Project conventions
- Minimal changes
- Error handling
- Documentation

## Response

```
Switching to coder mode.

[If RECALL has relevant patterns, mention them]

Ready to implement. What are we building?
```
