// Package tasks implements task synchronization between EDI and Claude Code.
//
// # Task Sync Strategy
//
// Tasks are synchronized at two key points:
//
//   - On launch: Load manifest from .edi/tasks/active.yaml, hydrate Claude Code's
//     task store at ~/.claude/tasks/{sessionID}/, and update the manifest with
//     the new session ID.
//
//   - On hook: Re-sync if session changed. The SessionStart hook triggers this
//     when Claude Code starts a new session within an EDI project.
//
// # Reconciliation
//
// When reconciling tasks between the EDI manifest and Claude Code's store:
//
//   - Newer timestamps win: If a task was modified more recently in Claude Code,
//     those changes are preserved.
//
//   - Completed tasks are removed: Tasks marked as "completed" or "done" are
//     automatically removed from the active manifest to keep the list clean.
//
//   - Only active tasks are hydrated: When hydrating a new session, only tasks
//     with status "pending" or "in_progress" are included.
//
// # File Format
//
// The manifest file (.edi/tasks/active.yaml) uses YAML format:
//
//	version: 1
//	last_session_id: abc123
//	last_sync: 2025-01-25T10:00:00Z
//	tasks:
//	  - id: "1"
//	    subject: "Implement feature X"
//	    description: "Full description..."
//	    status: pending
//	    blocks: ["2"]
//	    blocked_by: []
//
// # Migration
//
// The package supports migration from legacy manifest.yaml to active.yaml,
// automatically cleaning up completed tasks during migration.
//
// # Claude Code Integration
//
// Claude Code stores tasks in ~/.claude/tasks/{sessionID}/ as individual
// JSON files per task. This package converts between EDI's YAML manifest
// format and Claude Code's JSON task format.
//
// # Usage
//
//	// On EDI launch
//	sessionID, err := tasks.SyncOnLaunch("/path/to/project")
//
//	// On SessionStart hook
//	err := tasks.SyncOnHook("/path/to/project", sessionID)
//
//	// Load manifest directly
//	manifest, err := tasks.LoadManifest("/path/to/project")
//
//	// Cleanup old sessions
//	cleaned, err := tasks.CleanupOldSessions(24 * time.Hour)
package tasks
