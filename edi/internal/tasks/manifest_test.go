package tasks

import (
	"testing"
	"time"
)

func TestNewManifest(t *testing.T) {
	m := NewManifest()
	if m.Version != 1 {
		t.Errorf("Expected version 1, got %d", m.Version)
	}
	if len(m.Tasks) != 0 {
		t.Errorf("Expected empty tasks, got %d", len(m.Tasks))
	}
}

func TestManifestFindTask(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1"},
		{ID: "2", Subject: "Task 2"},
	}

	task := m.FindTask("1")
	if task == nil {
		t.Fatal("Expected to find task 1")
	}
	if task.Subject != "Task 1" {
		t.Errorf("Expected 'Task 1', got '%s'", task.Subject)
	}

	task = m.FindTask("3")
	if task != nil {
		t.Error("Expected nil for non-existent task")
	}
}

func TestManifestUpsertTask(t *testing.T) {
	m := NewManifest()

	// Add new task
	m.UpsertTask(Task{ID: "1", Subject: "Task 1"})
	if len(m.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(m.Tasks))
	}

	// Update existing task
	m.UpsertTask(Task{ID: "1", Subject: "Updated Task 1"})
	if len(m.Tasks) != 1 {
		t.Errorf("Expected 1 task after update, got %d", len(m.Tasks))
	}
	if m.Tasks[0].Subject != "Updated Task 1" {
		t.Errorf("Expected 'Updated Task 1', got '%s'", m.Tasks[0].Subject)
	}

	// Add another task
	m.UpsertTask(Task{ID: "2", Subject: "Task 2"})
	if len(m.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(m.Tasks))
	}
}

func TestManifestStats(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Status: "completed"},
		{ID: "2", Status: "completed"},
		{ID: "3", Status: "in_progress"},
		{ID: "4", Status: "pending"},
		{ID: "5", Status: "pending"},
		{ID: "6", Status: "pending"},
	}

	total, completed, inProgress, pending := m.Stats()
	if total != 6 {
		t.Errorf("Expected total 6, got %d", total)
	}
	if completed != 2 {
		t.Errorf("Expected completed 2, got %d", completed)
	}
	if inProgress != 1 {
		t.Errorf("Expected in_progress 1, got %d", inProgress)
	}
	if pending != 3 {
		t.Errorf("Expected pending 3, got %d", pending)
	}
}

func TestClaudeTaskToTask(t *testing.T) {
	ct := &ClaudeTask{
		ID:          "1",
		Subject:     "Test Task",
		Description: "Test Description",
		Status:      "in_progress",
		Blocks:      []string{"2"},
		BlockedBy:   []string{"3"},
	}

	modTime := time.Now()
	task := ct.ToTask(modTime)

	if task.ID != "1" {
		t.Errorf("Expected ID '1', got '%s'", task.ID)
	}
	if task.Subject != "Test Task" {
		t.Errorf("Expected Subject 'Test Task', got '%s'", task.Subject)
	}
	if task.Status != "in_progress" {
		t.Errorf("Expected Status 'in_progress', got '%s'", task.Status)
	}
	if len(task.Blocks) != 1 || task.Blocks[0] != "2" {
		t.Errorf("Expected Blocks ['2'], got %v", task.Blocks)
	}
}

func TestTaskToClaudeTask(t *testing.T) {
	task := &Task{
		ID:          "1",
		Subject:     "Test Task",
		Description: "Test Description",
		ActiveForm:  "Testing task",
		Status:      "pending",
		Blocks:      []string{"2"},
	}

	ct := task.ToClaudeTask()

	if ct.ID != "1" {
		t.Errorf("Expected ID '1', got '%s'", ct.ID)
	}
	if ct.Subject != "Test Task" {
		t.Errorf("Expected Subject 'Test Task', got '%s'", ct.Subject)
	}
	if ct.ActiveForm != "Testing task" {
		t.Errorf("Expected ActiveForm 'Testing task', got '%s'", ct.ActiveForm)
	}
}

func TestManifestTasksByStatus(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Status: "completed"},
		{ID: "2", Status: "in_progress"},
		{ID: "3", Status: "pending"},
		{ID: "4", Status: "pending"},
	}

	pending := m.TasksByStatus("pending")
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(pending))
	}

	completed := m.TasksByStatus("completed")
	if len(completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(completed))
	}
}

func TestManifestRemoveTask(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1"},
		{ID: "2", Subject: "Task 2"},
		{ID: "3", Subject: "Task 3"},
	}

	// Remove existing task
	removed := m.RemoveTask("2")
	if !removed {
		t.Error("Expected RemoveTask to return true for existing task")
	}
	if len(m.Tasks) != 2 {
		t.Errorf("Expected 2 tasks after removal, got %d", len(m.Tasks))
	}
	if m.FindTask("2") != nil {
		t.Error("Task 2 should be removed")
	}
	if m.FindTask("1") == nil || m.FindTask("3") == nil {
		t.Error("Tasks 1 and 3 should still exist")
	}

	// Remove non-existent task
	removed = m.RemoveTask("99")
	if removed {
		t.Error("Expected RemoveTask to return false for non-existent task")
	}
	if len(m.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(m.Tasks))
	}
}

func TestManifestRemoveCompletedTasks(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1", Status: "pending"},
		{ID: "2", Subject: "Task 2", Status: "completed"},
		{ID: "3", Subject: "Task 3", Status: "in_progress"},
		{ID: "4", Subject: "Task 4", Status: "done"},
		{ID: "5", Subject: "Task 5", Status: "pending"},
	}

	removed := m.RemoveCompletedTasks()
	if removed != 2 {
		t.Errorf("Expected 2 tasks removed, got %d", removed)
	}
	if len(m.Tasks) != 3 {
		t.Errorf("Expected 3 tasks remaining, got %d", len(m.Tasks))
	}

	// Verify only active tasks remain
	for _, task := range m.Tasks {
		if task.Status == "completed" || task.Status == "done" {
			t.Errorf("Task %s with status %s should have been removed", task.ID, task.Status)
		}
	}
}

func TestManifestActiveTasks(t *testing.T) {
	m := NewManifest()
	m.Tasks = []Task{
		{ID: "1", Subject: "Task 1", Status: "pending"},
		{ID: "2", Subject: "Task 2", Status: "completed"},
		{ID: "3", Subject: "Task 3", Status: "in_progress"},
		{ID: "4", Subject: "Task 4", Status: "done"},
	}

	active := m.ActiveTasks()
	if len(active) != 2 {
		t.Errorf("Expected 2 active tasks, got %d", len(active))
	}

	// Verify original manifest is unchanged
	if len(m.Tasks) != 4 {
		t.Errorf("Expected original manifest to have 4 tasks, got %d", len(m.Tasks))
	}

	// Verify active tasks have correct statuses
	for _, task := range active {
		if task.Status == "completed" || task.Status == "done" {
			t.Errorf("Active task %s should not have status %s", task.ID, task.Status)
		}
	}
}
