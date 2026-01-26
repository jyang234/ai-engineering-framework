// task-sync-hook is a lightweight CLI that syncs tasks on SessionStart
// It hydrates Claude Code's task store from the EDI manifest
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/aef/edi/internal/tasks"
)

// HookInput represents the JSON input from Claude Code's hook system
type HookInput struct {
	SessionID        string `json:"sessionId"`
	WorkingDirectory string `json:"workingDirectory"`
}

func main() {
	// Read hook input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Silent failure - don't break Claude Code
		os.Exit(0)
	}

	var hookInput HookInput
	if err := json.Unmarshal(input, &hookInput); err != nil {
		// If we can't parse input, try to get session ID another way
		// and use PWD for project path
	}

	// Determine project path
	projectPath := hookInput.WorkingDirectory
	if projectPath == "" {
		projectPath, _ = os.Getwd()
	}

	// Skip if not an EDI project
	if _, err := os.Stat(projectPath + "/.edi"); os.IsNotExist(err) {
		os.Exit(0)
	}

	// Determine session ID
	sessionID := hookInput.SessionID
	if sessionID == "" {
		// Try to detect from most recent session
		sessionID, _ = tasks.GetCurrentSessionID()
	}

	if sessionID == "" {
		// Can't determine session ID - skip sync
		os.Exit(0)
	}

	// Perform lightweight task sync
	if err := tasks.SyncOnHook(projectPath, sessionID); err != nil {
		// Log error but don't fail the hook
		fmt.Fprintf(os.Stderr, "task-sync-hook: %v\n", err)
	}

	// Output empty JSON to indicate success (no additional context needed)
	fmt.Println("{}")
}
