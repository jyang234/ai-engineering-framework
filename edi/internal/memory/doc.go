// Package memory manages Claude Code's auto memory (MEMORY.md) integration.
//
// Auto memory is a built-in Claude Code feature that persists a MEMORY.md file
// at ~/.claude/projects/<sanitized-project-path>/memory/. Its contents are
// automatically loaded into the system prompt at the start of every session.
// Claude Code also writes to MEMORY.md automatically during sessions.
//
// EDI co-manages MEMORY.md using a merge-based approach:
//   - EDI-managed sections (Project Quick Reference, Current State, promoted
//     RECALL items, Topic Index) are regenerated on each launch
//   - Non-EDI sections (EDI Observations, Claude auto-saved memories, user
//     additions) are preserved across regenerations
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
package memory
