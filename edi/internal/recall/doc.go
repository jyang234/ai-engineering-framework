// Package recall implements the RECALL MCP server for organizational knowledge.
//
// RECALL provides tools for storing and retrieving patterns, decisions, and
// failures. It uses SQLite with FTS5 for full-text search.
//
// # Architecture
//
// The package consists of two main components:
//
//   - Storage: SQLite database operations using FTS5 for full-text search.
//     Handles knowledge items (patterns, decisions, failures) and flight
//     recorder entries.
//
//   - Server: MCP (Model Context Protocol) server that exposes RECALL tools
//     to Claude Code via JSON-RPC 2.0 over stdio.
//
// # MCP Tools
//
// The server exposes the following tools:
//
//   - recall_search: Search organizational knowledge for patterns, failures,
//     and decisions using full-text search.
//
//   - recall_get: Retrieve a specific knowledge item by ID.
//
//   - recall_add: Add new knowledge (pattern, failure, or decision) to RECALL.
//
//   - recall_feedback: Provide feedback on whether a RECALL item was useful,
//     which helps improve ranking over time.
//
//   - flight_recorder_log: Log decisions, errors, and milestones during work
//     for later review and potential capture to RECALL.
//
// # Item Types
//
// Knowledge items have the following types:
//
//   - pattern: Reusable solutions, code patterns, or best practices
//   - failure: Known issues, bugs, and their solutions
//   - decision: Architecture Decision Records (ADRs) and technical choices
//   - context: Background information about systems or domains
//
// # Scopes
//
// Items can be scoped as:
//
//   - global: Available across all projects (stored in ~/.edi/recall/)
//   - project: Specific to a single project (stored in .edi/recall/)
//
// # Usage
//
//	storage, err := recall.NewStorage("/path/to/db.sqlite")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer storage.Close()
//
//	server := recall.NewServer(storage, "session-123")
//	if err := server.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
package recall
