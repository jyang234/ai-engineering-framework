---
name: end-recovery
description: Recover a summary from a previous session that was not cleanly ended
---

# Session Recovery

The previous session was not cleanly ended (likely exited via Ctrl+C).
Generate a best-effort recovery summary.

## Steps

1. **Acknowledge** this is a recovery for a previous session — we don't have the conversation history.

2. **Ask the user** what they remember working on:
   ```
   The previous session wasn't cleanly ended. I'll generate a recovery summary.
   What were you working on? (Brief description is fine, or say "skip" to use git history only)
   ```

3. **Gather context** from git:
   - Run `git log --oneline -20` to see recent commits
   - Run `git diff --stat HEAD~5` (or appropriate range) to see recent changes
   - Check `.edi/status.md` for last known status

4. **Generate summary** combining user input and git context

5. **Update** `.edi/status.md` with current project status:
   - Read the existing `.edi/status.md` (if any)
   - Update it based on recovered context
   - Include a `Last updated: {current date}` line at the top

6. **Write** session history to `.edi/history/{date}-{session-id}.md`:
   ```markdown
   ---
   session_id: [recovered session ID from context]
   started_at: [last sync time if available]
   ended_at: [current time]
   agent: recovery
   ---

   # Session Summary (Recovered)

   > This summary was recovered after an unclean exit.

   ## Accomplished
   - [bullet points based on git history and user input]

   ## Context
   - [any relevant context from git log/diff]
   ```

7. **Confirm** recovery complete:
   ```
   Recovery saved to .edi/history/{filename}
   Previous session context has been preserved.
   ```
