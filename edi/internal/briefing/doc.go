// Package briefing generates session context for Claude Code.
//
// A briefing provides Claude Code with context about the current project and
// recent work, enabling more informed and consistent assistance across sessions.
//
// # Briefing Components
//
// A briefing includes:
//
//   - Project profile: Content from .edi/profile.md describing the project's
//     purpose, architecture, tech stack, conventions, and key decisions.
//
//   - Recent session history: Summaries from .edi/history/ showing what was
//     accomplished in recent sessions.
//
//   - Current task status: Overview of pending and in-progress tasks from
//     the Claude Code task system.
//
// # Rendered Format
//
// The briefing is rendered as Markdown suitable for system prompt injection:
//
//	# EDI Briefing: project-name
//
//	## Project Context
//	[profile content]
//
//	## Recent Sessions
//	### 2025-01-24 (1d ago)
//	- Implemented feature X
//	- Fixed bug Y
//
//	## Current Tasks
//	**Status**: 2 completed, 1 in progress, 3 pending
//
//	**Ready to Start:**
//	- Task 4: Implement Z
//
// # History Management
//
// Session history is stored in .edi/history/ as Markdown files with YAML
// frontmatter:
//
//	---
//	session_id: abc12345
//	started_at: 2025-01-25T10:00:00Z
//	ended_at: 2025-01-25T12:00:00Z
//	agent: coder
//	tasks_completed:
//	  - Implement feature X
//	decisions_captured:
//	  - Use REST over GraphQL
//	---
//
//	# Session Summary
//	[summary content]
//
// # Flight Recorder
//
// The package also provides FlightRecorderFile for logging decisions, errors,
// and milestones during a session. These are stored as JSONL files in
// .edi/history/{session-prefix}-flight.jsonl.
//
// # Usage
//
//	// Generate a briefing
//	brief, err := briefing.Generate(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Render for system prompt
//	rendered := brief.Render("my-project")
//
//	// Save session history
//	entry := &briefing.HistoryEntry{
//	    SessionID: sessionID,
//	    Date:      time.Now(),
//	    Summary:   "Implemented X, fixed Y",
//	}
//	err := briefing.SaveHistory(projectPath, entry)
//
//	// Create flight recorder
//	fr, err := briefing.NewFlightRecorderFile(projectPath, sessionID)
//	defer fr.Close()
//	fr.Write([]byte(`{"type":"decision","content":"Use caching"}`))
package briefing
