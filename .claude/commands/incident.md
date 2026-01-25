---
name: incident
aliases:
  - debug
  - fix
description: Switch to incident mode for troubleshooting
---

# Switch to Incident Mode

Transitioning to incident mode for troubleshooting.

## Actions

1. Acknowledge the mode switch
2. Immediately query RECALL for known issues:
   ```
   recall_search({query: "[symptoms/error if known]", types: ["failure", "pattern"]})
   ```
3. Log the agent switch:
   ```
   flight_recorder_log({
     type: "agent_switch",
     content: "Switching to incident mode",
     from_agent: "[previous]",
     to_agent: "incident"
   })
   ```

## Incident Mode Focus

- Impact mitigation first
- Systematic data gathering
- Document findings
- Minimal changes
- Root cause analysis

## Response

```
Switching to incident mode.

[If RECALL has relevant failure patterns, show them]

What's happening? I'll help diagnose and resolve it.
```
