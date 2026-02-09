// Package memory manages Claude Code's auto memory (MEMORY.md) integration.
//
// Auto memory is a built-in Claude Code feature that persists a MEMORY.md file
// at ~/.claude/projects/<sanitized-project-path>/memory/. Its contents are
// automatically loaded into the system prompt at the start of every session.
//
// EDI manages MEMORY.md to create a layered memory architecture:
//
//	L1: MEMORY.md      — Always loaded, ~150 lines, highest-signal content
//	L2: EDI Context    — Session-specific: agent mode, tasks, RECALL instructions
//	L3: RECALL DB      — Deep searchable knowledge (patterns, decisions, failures)
//
// MEMORY.md acts as a curated cache for RECALL's highest-value insights. EDI
// generates and updates it on launch and at session end (/end).
package memory
