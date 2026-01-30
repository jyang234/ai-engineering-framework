---
name: end
description: End the current EDI session
---

# End Session

Generate a session summary and save it to history.

## Steps

1. **Summarize** what was accomplished this session:
   - Tasks completed
   - Code changes made
   - Decisions reached

2. **List** key decisions made with their rationale

3. **Identify** capture candidates - things worth saving to RECALL:
   - New patterns discovered
   - Failures encountered and fixed
   - Important decisions with rationale

4. **Present** capture candidates to user:
   ```
   I identified these items worth capturing to RECALL:

   [1] Pattern: [description]
   [2] Decision: [description]
   [3] Failure: [description]

   Capture to RECALL? [A]ll / [1-3] Select / [S]kip
   ```

5. **Save** approved items using `recall_add`:
   ```
   recall_add({
     type: "pattern",
     title: "[brief title]",
     content: "[full description with context]",
     tags: ["[relevant]", "[tags]"]
   })
   ```

6. **Update** `.edi/status.md` with current project status:
   - Read the existing `.edi/status.md` (if any)
   - Update it based on what was accomplished this session and what's next
   - Include a `Last updated: {current date}` line at the top
   - Keep it concise: current milestone, what's done, what's next
   ```markdown
   Last updated: 2026-01-29

   ## Current Milestone
   [What you're working toward]

   ## Completed
   - [Recent completions]

   ## Next Steps
   - [What comes next]
   ```

7. **Write** session history to `.edi/history/{date}-{session-id}.md`:
   ```markdown
   ---
   session_id: [full session ID from context]
   started_at: [session start time]
   ended_at: [current time]
   agent: [current agent mode]
   tasks_completed: [list of task IDs]
   decisions_captured: [list of RECALL IDs from this session]
   ---

   # Session Summary

   ## Accomplished
   - [bullet points of completed work]

   ## Key Decisions
   - [decisions with brief rationale]

   ## Open Items
   - [work remaining, blockers]
   ```

8. **Confirm** session ended:
   ```
   Session saved to .edi/history/2026-01-25-abc12345.md
   Captured 2 items to RECALL.
   ```
