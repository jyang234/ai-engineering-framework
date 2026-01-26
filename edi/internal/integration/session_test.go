//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/aef/edi/internal/briefing"
	"github.com/anthropics/aef/edi/internal/config"
	"github.com/anthropics/aef/edi/internal/launch"
	"github.com/anthropics/aef/edi/internal/tasks"
	"github.com/anthropics/aef/edi/internal/testutil"
)

func TestLaunchContextBuilding(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Setup project structure
	profileContent := `# Test Project

## Overview

A test project for integration testing EDI session lifecycle.

## Tech Stack

- Go 1.22+
- SQLite with FTS5
- Cobra CLI framework

## Key Patterns

- Standard Go project layout
- Embedded assets with //go:embed
`
	env.CreateProjectFile(".edi/profile.md", profileContent)
	env.CreateProjectFile(".edi/config.yaml", `version: 1
briefing:
  include_profile: true
  include_tasks: true
`)

	// Create tasks
	env.CreateEDITasksDir()
	env.CreateProjectFile(".edi/tasks/active.yaml", `version: 1
last_session_id: old-session-123
tasks:
  - id: "1"
    subject: Implement feature A
    description: Add feature A to the system
    status: pending
  - id: "2"
    subject: Write tests for feature A
    description: Create comprehensive test suite
    status: pending
    blockedBy:
      - "1"
`)

	// Change to project directory for context building
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(env.ProjectDir)

	// Test: Generate briefing
	t.Run("GenerateBriefing", func(t *testing.T) {
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		brief, err := briefing.Generate(cfg)
		if err != nil {
			t.Fatalf("Failed to generate briefing: %v", err)
		}

		if !brief.HasProfile {
			t.Error("Expected briefing to have profile")
		}
		if !strings.Contains(brief.ProjectContext, "Test Project") {
			t.Error("Expected profile content in briefing")
		}
		if !brief.HasTasks {
			t.Error("Expected briefing to have tasks")
		}
		if brief.CurrentTasks == nil {
			t.Fatal("Expected CurrentTasks to be set")
		}
		if brief.CurrentTasks.Pending != 2 {
			t.Errorf("Expected 2 pending tasks, got %d", brief.CurrentTasks.Pending)
		}
	})

	// Test: Build launch context
	t.Run("BuildContext", func(t *testing.T) {
		cfg, _ := config.Load()
		brief, _ := briefing.Generate(cfg)

		contextPath, err := launch.BuildContext(cfg, "test-session-456", brief, "test-project")
		if err != nil {
			t.Fatalf("Failed to build context: %v", err)
		}
		defer os.Remove(contextPath)

		// Read and verify context file
		content, err := os.ReadFile(contextPath)
		if err != nil {
			t.Fatalf("Failed to read context file: %v", err)
		}

		contextStr := string(content)

		// Check for EDI identity
		if !strings.Contains(contextStr, "EDI - Enhanced Development Intelligence") {
			t.Error("Expected EDI identity header in context")
		}

		// Check for session ID
		if !strings.Contains(contextStr, "test-session-456") {
			t.Error("Expected session ID in context")
		}

		// Check for profile content
		if !strings.Contains(contextStr, "Test Project") {
			t.Error("Expected profile content in context")
		}

		// Check for task summary
		if !strings.Contains(contextStr, "pending") {
			t.Error("Expected task status in context")
		}

		// Check for slash commands
		if !strings.Contains(contextStr, "/plan") {
			t.Error("Expected slash command instructions in context")
		}
	})

	// Test: Context includes RECALL instructions when enabled
	t.Run("ContextIncludesRecallWhenEnabled", func(t *testing.T) {
		cfg, _ := config.Load()
		cfg.Recall.Enabled = true

		brief, _ := briefing.Generate(cfg)
		contextPath, err := launch.BuildContext(cfg, "test-session-789", brief, "test-project")
		if err != nil {
			t.Fatalf("Failed to build context: %v", err)
		}
		defer os.Remove(contextPath)

		content, _ := os.ReadFile(contextPath)
		contextStr := string(content)

		if !strings.Contains(contextStr, "RECALL Knowledge Base") {
			t.Error("Expected RECALL section when enabled")
		}
		if !strings.Contains(contextStr, "recall_search") {
			t.Error("Expected recall tools listed")
		}
	})
}

func TestTaskSyncOnLaunch(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Setup project with tasks
	env.SetupMinimalProject()
	env.CreateEDITasksDir()
	env.CreateProjectFile(".edi/tasks/active.yaml", `version: 1
last_session_id: old-session-abc
tasks:
  - id: "task-001"
    subject: Implement MCP client
    description: Create test client for MCP protocol
    status: pending
    activeForm: Implementing MCP client
  - id: "task-002"
    subject: Write integration tests
    description: Integration tests for RECALL server
    status: in_progress
    blockedBy:
      - "task-001"
  - id: "task-003"
    subject: Completed task
    description: This task is already done
    status: completed
`)

	// Test: SyncOnLaunch creates session and hydrates tasks
	t.Run("SyncOnLaunchCreatesSession", func(t *testing.T) {
		sessionID, err := tasks.SyncOnLaunch(env.ProjectDir)
		if err != nil {
			t.Fatalf("SyncOnLaunch failed: %v", err)
		}

		if sessionID == "" {
			t.Error("Expected non-empty session ID")
		}

		// Verify manifest was updated
		m, err := tasks.LoadManifest(env.ProjectDir)
		if err != nil {
			t.Fatalf("Failed to load manifest: %v", err)
		}

		if m.LastSessionID != sessionID {
			t.Errorf("Expected last_session_id '%s', got '%s'", sessionID, m.LastSessionID)
		}
	})

	// Test: Tasks are hydrated to Claude store
	t.Run("TasksHydratedToClaudeStore", func(t *testing.T) {
		// Load manifest to get session ID
		m, _ := tasks.LoadManifest(env.ProjectDir)
		sessionDir := filepath.Join(env.Home, ".claude", "tasks", m.LastSessionID)

		// Check that active tasks were hydrated
		entries, err := os.ReadDir(sessionDir)
		if err != nil {
			t.Fatalf("Failed to read session directory: %v", err)
		}

		// Should have 2 task files (pending and in_progress, not completed)
		if len(entries) != 2 {
			t.Errorf("Expected 2 active task files, got %d", len(entries))
		}

		// Verify completed task was NOT hydrated
		completedPath := filepath.Join(sessionDir, "task-003.json")
		if _, err := os.Stat(completedPath); !os.IsNotExist(err) {
			t.Error("Completed task should not be hydrated")
		}
	})
}

func TestTaskSyncOnHook(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Setup project with tasks
	env.SetupMinimalProject()
	env.CreateEDITasksDir()
	env.CreateProjectFile(".edi/tasks/active.yaml", `version: 1
last_session_id: initial-session
tasks:
  - id: "hook-task-1"
    subject: Task for hook test
    description: Testing hook-based sync
    status: pending
`)

	// Test: SyncOnHook updates session ID and hydrates
	t.Run("SyncOnHookUpdatesSession", func(t *testing.T) {
		newSessionID := "new-hook-session-123"

		err := tasks.SyncOnHook(env.ProjectDir, newSessionID)
		if err != nil {
			t.Fatalf("SyncOnHook failed: %v", err)
		}

		// Verify manifest was updated
		m, err := tasks.LoadManifest(env.ProjectDir)
		if err != nil {
			t.Fatalf("Failed to load manifest: %v", err)
		}

		if m.LastSessionID != newSessionID {
			t.Errorf("Expected last_session_id '%s', got '%s'", newSessionID, m.LastSessionID)
		}

		// Verify tasks were hydrated to new session
		sessionDir := filepath.Join(env.Home, ".claude", "tasks", newSessionID)
		entries, err := os.ReadDir(sessionDir)
		if err != nil {
			t.Fatalf("Failed to read session directory: %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("Expected 1 task file in new session, got %d", len(entries))
		}
	})
}

func TestBriefingRender(t *testing.T) {
	// Test: Briefing renders correctly with all sections
	t.Run("FullBriefingRender", func(t *testing.T) {
		brief := &briefing.Briefing{
			HasProfile: true,
			ProjectContext: `# My Project

A sample project for testing.
`,
			HasTasks: true,
			CurrentTasks: &briefing.TaskStatus{
				Total:      3,
				Completed:  1,
				InProgress: 1,
				Pending:    1,
				InProgressItems: []briefing.TaskItem{
					{ID: "1", Description: "Working on feature X"},
				},
				ReadyItems: []briefing.TaskItem{
					{ID: "2", Description: "Ready to implement Y"},
				},
			},
		}

		rendered := brief.Render("test-project")

		// Check header
		if !strings.Contains(rendered, "# EDI Briefing: test-project") {
			t.Error("Expected briefing header with project name")
		}

		// Check project context
		if !strings.Contains(rendered, "## Project Context") {
			t.Error("Expected Project Context section")
		}
		if !strings.Contains(rendered, "My Project") {
			t.Error("Expected project content")
		}

		// Check task summary
		if !strings.Contains(rendered, "## Current Tasks") {
			t.Error("Expected Current Tasks section")
		}
		if !strings.Contains(rendered, "1 completed") {
			t.Error("Expected completed count")
		}
		if !strings.Contains(rendered, "1 in progress") {
			t.Error("Expected in progress count")
		}
		if !strings.Contains(rendered, "1 pending") {
			t.Error("Expected pending count")
		}

		// Check in-progress items
		if !strings.Contains(rendered, "**In Progress:**") {
			t.Error("Expected In Progress section")
		}
		if !strings.Contains(rendered, "Working on feature X") {
			t.Error("Expected in-progress task description")
		}

		// Check ready items
		if !strings.Contains(rendered, "**Ready to Start:**") {
			t.Error("Expected Ready to Start section")
		}
		if !strings.Contains(rendered, "Ready to implement Y") {
			t.Error("Expected ready task description")
		}

		// Check closing
		if !strings.Contains(rendered, "Ready to continue. What would you like to work on?") {
			t.Error("Expected closing prompt")
		}
	})

	// Test: Empty briefing still renders header and closing
	t.Run("EmptyBriefingRender", func(t *testing.T) {
		brief := &briefing.Briefing{}

		rendered := brief.Render("empty-project")

		if !strings.Contains(rendered, "# EDI Briefing: empty-project") {
			t.Error("Expected briefing header even for empty briefing")
		}
		if !strings.Contains(rendered, "Ready to continue") {
			t.Error("Expected closing even for empty briefing")
		}
	})
}
