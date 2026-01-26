package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSaveManifest(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .edi directory
	ediDir := filepath.Join(tmpDir, ".edi")
	if err := os.MkdirAll(ediDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Load from non-existent file should return empty manifest
	m, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("Expected version 1, got %d", m.Version)
	}

	// Add some tasks and save
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1", Status: "pending", CreatedAt: time.Now()},
		{ID: "2", Subject: "Task 2", Status: "completed", CreatedAt: time.Now()},
	}
	m.LastSessionID = "test-session"

	if err := SaveManifest(tmpDir, m); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest after save failed: %v", err)
	}

	if len(loaded.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(loaded.Tasks))
	}
	if loaded.LastSessionID != "test-session" {
		t.Errorf("Expected last_session_id 'test-session', got '%s'", loaded.LastSessionID)
	}
}

func TestHydrateClaudeStore(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create tasks to hydrate
	tasks := []Task{
		{
			ID:          "1",
			Subject:     "Task 1",
			Description: "Description 1",
			Status:      "pending",
		},
		{
			ID:          "2",
			Subject:     "Task 2",
			Description: "Description 2",
			Status:      "in_progress",
			Blocks:      []string{"3"},
		},
	}

	sessionID := "test-session-123"
	if err := HydrateClaudeStore(sessionID, tasks); err != nil {
		t.Fatalf("HydrateClaudeStore failed: %v", err)
	}

	// Verify files were created
	sessionDir := filepath.Join(tmpHome, ".claude", "tasks", sessionID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("Failed to read session directory: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 task files, got %d", len(entries))
	}

	// Verify task content
	taskPath := filepath.Join(sessionDir, "1.json")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("Failed to read task file: %v", err)
	}

	var ct ClaudeTask
	if err := json.Unmarshal(data, &ct); err != nil {
		t.Fatalf("Failed to parse task JSON: %v", err)
	}

	if ct.Subject != "Task 1" {
		t.Errorf("Expected subject 'Task 1', got '%s'", ct.Subject)
	}
}

func TestReconcileTasks(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	m := NewManifest()
	m.LastSync = time.Now().Add(-time.Hour)

	// Add an existing task to manifest
	m.Tasks = []Task{
		{
			ID:        "1",
			Subject:   "Old Task 1",
			Status:    "pending",
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		},
	}

	// Create Claude tasks (simulating newer updates)
	sessionTasks := map[string][]ClaudeTask{
		"session-1": {
			{
				ID:      "1",
				Subject: "Updated Task 1",
				Status:  "in_progress",
			},
			{
				ID:      "2",
				Subject: "New Task 2",
				Status:  "pending",
			},
		},
	}

	// Create task files with recent mod times
	sessionDir := filepath.Join(tmpHome, ".claude", "tasks", "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, ct := range sessionTasks["session-1"] {
		data, _ := json.Marshal(ct)
		if err := os.WriteFile(filepath.Join(sessionDir, ct.ID+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	removed := ReconcileTasks(m, sessionTasks)

	// No completed tasks, so none removed
	if removed != 0 {
		t.Errorf("Expected 0 tasks removed, got %d", removed)
	}

	// Should have 2 tasks now
	if len(m.Tasks) != 2 {
		t.Errorf("Expected 2 tasks after reconciliation, got %d", len(m.Tasks))
	}

	// Task 1 should be updated (newer mod time wins)
	task1 := m.FindTask("1")
	if task1 == nil {
		t.Fatal("Task 1 should exist")
	}

	// Task 2 should be added
	task2 := m.FindTask("2")
	if task2 == nil {
		t.Fatal("Task 2 should be added")
	}
	if task2.Subject != "New Task 2" {
		t.Errorf("Expected 'New Task 2', got '%s'", task2.Subject)
	}
}

func TestReconcileTasksRemovesCompleted(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	m := NewManifest()
	m.LastSync = time.Now().Add(-time.Hour)

	// Add an existing task to manifest
	m.Tasks = []Task{
		{
			ID:        "1",
			Subject:   "Task 1",
			Status:    "pending",
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		},
	}

	// Create Claude tasks with one completed
	sessionTasks := map[string][]ClaudeTask{
		"session-1": {
			{
				ID:      "1",
				Subject: "Task 1",
				Status:  "completed", // Now completed
			},
			{
				ID:      "2",
				Subject: "New Task 2",
				Status:  "pending",
			},
			{
				ID:      "3",
				Subject: "Already Done",
				Status:  "done", // Completed task should not be added
			},
		},
	}

	// Create task files with recent mod times
	sessionDir := filepath.Join(tmpHome, ".claude", "tasks", "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, ct := range sessionTasks["session-1"] {
		data, _ := json.Marshal(ct)
		if err := os.WriteFile(filepath.Join(sessionDir, ct.ID+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	removed := ReconcileTasks(m, sessionTasks)

	// Task 1 was marked completed and should be removed
	if removed != 1 {
		t.Errorf("Expected 1 task removed, got %d", removed)
	}

	// Should only have 1 active task (Task 2)
	if len(m.Tasks) != 1 {
		t.Errorf("Expected 1 task after reconciliation, got %d", len(m.Tasks))
	}

	// Task 2 should be the only one
	task2 := m.FindTask("2")
	if task2 == nil {
		t.Fatal("Task 2 should exist")
	}

	// Task 1 should be removed (was completed)
	task1 := m.FindTask("1")
	if task1 != nil {
		t.Error("Task 1 should be removed after completion")
	}

	// Task 3 should not have been added (already completed)
	task3 := m.FindTask("3")
	if task3 != nil {
		t.Error("Task 3 should not be added (already completed)")
	}
}

func TestSyncOnLaunchNotEdiProject(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// No .edi directory - should return empty session ID
	sessionID, err := SyncOnLaunch(tmpDir)
	if err != nil {
		t.Fatalf("SyncOnLaunch should not error for non-EDI project: %v", err)
	}
	if sessionID != "" {
		t.Errorf("Expected empty session ID for non-EDI project, got '%s'", sessionID)
	}
}

func TestSyncOnLaunchEdiProject(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create .edi directory
	ediDir := filepath.Join(tmpDir, ".edi")
	if err := os.MkdirAll(ediDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First run - should create new session
	sessionID, err := SyncOnLaunch(tmpDir)
	if err != nil {
		t.Fatalf("SyncOnLaunch failed: %v", err)
	}
	if sessionID == "" {
		t.Error("Expected non-empty session ID")
	}

	// Verify manifest was created
	m, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}
	if m.LastSessionID != sessionID {
		t.Errorf("Expected last_session_id '%s', got '%s'", sessionID, m.LastSessionID)
	}
}

func TestSyncOnHook(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create .edi directory and manifest with tasks
	ediDir := filepath.Join(tmpDir, ".edi")
	if err := os.MkdirAll(ediDir, 0755); err != nil {
		t.Fatal(err)
	}

	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1", Status: "pending"},
		{ID: "2", Subject: "Task 2", Status: "in_progress"},
	}
	m.LastSessionID = "old-session"
	if err := SaveManifest(tmpDir, m); err != nil {
		t.Fatal(err)
	}

	// Run hook with new session ID
	newSessionID := "new-session-123"
	if err := SyncOnHook(tmpDir, newSessionID); err != nil {
		t.Fatalf("SyncOnHook failed: %v", err)
	}

	// Verify tasks were hydrated to new session
	sessionDir := filepath.Join(tmpHome, ".claude", "tasks", newSessionID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("Failed to read session directory: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 task files, got %d", len(entries))
	}

	// Verify manifest was updated
	m, _ = LoadManifest(tmpDir)
	if m.LastSessionID != newSessionID {
		t.Errorf("Expected last_session_id '%s', got '%s'", newSessionID, m.LastSessionID)
	}
}

func TestSyncOnHookOnlyActiveTasksHydrated(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create .edi directory and manifest with mixed tasks
	ediDir := filepath.Join(tmpDir, ".edi")
	if err := os.MkdirAll(ediDir, 0755); err != nil {
		t.Fatal(err)
	}

	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Active Task", Status: "pending"},
		{ID: "2", Subject: "Completed Task", Status: "completed"},
		{ID: "3", Subject: "In Progress", Status: "in_progress"},
	}
	m.LastSessionID = "old-session"
	if err := SaveManifest(tmpDir, m); err != nil {
		t.Fatal(err)
	}

	// Run hook with new session ID
	newSessionID := "new-session-456"
	if err := SyncOnHook(tmpDir, newSessionID); err != nil {
		t.Fatalf("SyncOnHook failed: %v", err)
	}

	// Verify only active tasks were hydrated
	sessionDir := filepath.Join(tmpHome, ".claude", "tasks", newSessionID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("Failed to read session directory: %v", err)
	}

	// Should only have 2 task files (pending and in_progress)
	if len(entries) != 2 {
		t.Errorf("Expected 2 active task files, got %d", len(entries))
	}

	// Verify completed task was not hydrated
	completedTaskPath := filepath.Join(sessionDir, "2.json")
	if _, err := os.Stat(completedTaskPath); !os.IsNotExist(err) {
		t.Error("Completed task should not be hydrated")
	}
}

func TestCleanupOldSessions(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	tasksDir := filepath.Join(tmpHome, ".claude", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some session directories
	oldSession := filepath.Join(tasksDir, "old-session")
	newSession := filepath.Join(tasksDir, "new-session")
	if err := os.MkdirAll(oldSession, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newSession, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a task file to each to make them non-empty
	if err := os.WriteFile(filepath.Join(oldSession, "1.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newSession, "1.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set old session's mod time to 48 hours ago
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldSession, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Cleanup sessions older than 24 hours
	cleaned, err := CleanupOldSessions(24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldSessions failed: %v", err)
	}

	if cleaned != 1 {
		t.Errorf("Expected 1 session cleaned, got %d", cleaned)
	}

	// Verify old session is gone
	if _, err := os.Stat(oldSession); !os.IsNotExist(err) {
		t.Error("Old session should be removed")
	}

	// Verify new session still exists
	if _, err := os.Stat(newSession); err != nil {
		t.Error("New session should still exist")
	}
}

func TestLoadManifestMigratesLegacyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .edi/tasks directory
	tasksDir := filepath.Join(tmpDir, ".edi", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create legacy manifest.yaml with mixed tasks
	legacyPath := filepath.Join(tasksDir, "manifest.yaml")
	legacyContent := `version: 1
last_session_id: old-session
tasks:
  - id: "1"
    subject: Active Task
    status: pending
  - id: "2"
    subject: Completed Task
    status: completed
  - id: "3"
    subject: Another Active
    status: in_progress
`
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load manifest (should migrate)
	m, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// Should only have active tasks (completed removed during migration)
	if len(m.Tasks) != 2 {
		t.Errorf("Expected 2 active tasks after migration, got %d", len(m.Tasks))
	}

	// Legacy file should be removed
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("Legacy manifest.yaml should be removed after migration")
	}

	// New active.yaml should exist
	newPath := filepath.Join(tasksDir, "active.yaml")
	if _, err := os.Stat(newPath); err != nil {
		t.Error("New active.yaml should exist after migration")
	}
}
