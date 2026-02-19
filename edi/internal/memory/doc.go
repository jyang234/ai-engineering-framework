// Package memory manages Claude Code's auto memory (MEMORY.md) integration.
//
// Auto memory is a built-in Claude Code feature that persists a MEMORY.md file
// at ~/.claude/projects/<sanitized-git-root>/memory/. The first 200 lines are
// loaded into the system prompt at the start of every session. Claude
// autonomously writes memories during sessions — users see "Wrote X memories"
// terminal messages. Claude decides what to record without being asked.
//
// EDI co-manages MEMORY.md using a merge-based approach:
//   - EDI-managed sections (Project Quick Reference, Current State, promoted
//     RECALL items, Topic Index) are regenerated on each launch
//   - Non-EDI sections (EDI Observations, Claude-written sections, user
//     additions, preamble content) are preserved across regenerations
//
// This creates a layered memory architecture:
//
//	L1: MEMORY.md      — Always loaded, ~150 lines, highest-signal content
//	L2: EDI Context    — Session-specific: agent mode, tasks, RECALL instructions
//	L3: RECALL DB      — Deep searchable knowledge (patterns, decisions, failures)
//
// MEMORY.md acts as a curated cache for RECALL's highest-value insights. EDI
// merges updates on launch and at session end (/end), preserving any content
// that Claude or the user wrote outside of EDI-managed sections.
//
// Note on line budget: enforceLineBudget truncates from the bottom of the file.
// Since preserved sections appear after EDI content, they are truncated first
// when the 195-line limit is hit. EDI's own content (bounded by slot budgets)
// has effective priority.
package memory
