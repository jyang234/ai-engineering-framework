package tasks

import (
	"encoding/json"
	"time"
)

// Manifest represents the project-scoped active task list
// stored in .edi/tasks/active.yaml (only contains non-completed tasks)
type Manifest struct {
	Version       int       `yaml:"version" json:"version"`
	LastSync      time.Time `yaml:"last_sync" json:"last_sync"`
	LastSessionID string    `yaml:"last_session_id" json:"last_session_id"`
	Tasks         []Task    `yaml:"tasks" json:"tasks"`
}

// Task represents a task in the manifest, matching Claude Code's task format
type Task struct {
	ID          string    `yaml:"id" json:"id"`
	Subject     string    `yaml:"subject" json:"subject"`
	Description string    `yaml:"description" json:"description"`
	ActiveForm  string    `yaml:"active_form,omitempty" json:"activeForm,omitempty"`
	Status      string    `yaml:"status" json:"status"`
	Owner       string    `yaml:"owner,omitempty" json:"owner,omitempty"`
	Blocks      []string  `yaml:"blocks,omitempty" json:"blocks,omitempty"`
	BlockedBy   []string  `yaml:"blocked_by,omitempty" json:"blockedBy,omitempty"`
	Metadata    Metadata  `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt   time.Time `yaml:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `yaml:"updated_at" json:"updatedAt"`
}

// Metadata holds arbitrary task metadata
type Metadata map[string]interface{}

// ClaudeTask represents the task format as stored by Claude Code
// in ~/.claude/tasks/{sessionId}/{taskId}.json
type ClaudeTask struct {
	ID          string                 `json:"id"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Status      string                 `json:"status"`
	Owner       string                 `json:"owner,omitempty"`
	Blocks      []string               `json:"blocks,omitempty"`
	BlockedBy   []string               `json:"blockedBy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToTask converts a ClaudeTask to a manifest Task
func (ct *ClaudeTask) ToTask(modTime time.Time) Task {
	return Task{
		ID:          ct.ID,
		Subject:     ct.Subject,
		Description: ct.Description,
		ActiveForm:  ct.ActiveForm,
		Status:      ct.Status,
		Owner:       ct.Owner,
		Blocks:      ct.Blocks,
		BlockedBy:   ct.BlockedBy,
		Metadata:    ct.Metadata,
		CreatedAt:   modTime, // Use file mod time as approximation
		UpdatedAt:   modTime,
	}
}

// ToClaudeTask converts a manifest Task to ClaudeTask format for hydration
func (t *Task) ToClaudeTask() *ClaudeTask {
	return &ClaudeTask{
		ID:          t.ID,
		Subject:     t.Subject,
		Description: t.Description,
		ActiveForm:  t.ActiveForm,
		Status:      t.Status,
		Owner:       t.Owner,
		Blocks:      t.Blocks,
		BlockedBy:   t.BlockedBy,
		Metadata:    t.Metadata,
	}
}

// MarshalJSON marshals ClaudeTask to JSON for writing to Claude's task store
func (ct *ClaudeTask) MarshalJSON() ([]byte, error) {
	type Alias ClaudeTask
	return json.Marshal((*Alias)(ct))
}

// NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Version: 1,
		Tasks:   []Task{},
	}
}

// FindTask finds a task by ID in the manifest
func (m *Manifest) FindTask(id string) *Task {
	for i := range m.Tasks {
		if m.Tasks[i].ID == id {
			return &m.Tasks[i]
		}
	}
	return nil
}

// UpsertTask adds or updates a task in the manifest
func (m *Manifest) UpsertTask(task Task) {
	for i := range m.Tasks {
		if m.Tasks[i].ID == task.ID {
			m.Tasks[i] = task
			return
		}
	}
	m.Tasks = append(m.Tasks, task)
}

// TasksByStatus returns tasks filtered by status
func (m *Manifest) TasksByStatus(status string) []Task {
	var result []Task
	for _, t := range m.Tasks {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// Stats returns task statistics
func (m *Manifest) Stats() (total, completed, inProgress, pending int) {
	for _, t := range m.Tasks {
		total++
		switch t.Status {
		case "completed", "done":
			completed++
		case "in_progress", "active":
			inProgress++
		default:
			pending++
		}
	}
	return
}

// RemoveTask removes a task by ID from the manifest
func (m *Manifest) RemoveTask(id string) bool {
	for i := range m.Tasks {
		if m.Tasks[i].ID == id {
			m.Tasks = append(m.Tasks[:i], m.Tasks[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveCompletedTasks removes all completed tasks from the manifest
// Returns the number of tasks removed
func (m *Manifest) RemoveCompletedTasks() int {
	active := make([]Task, 0, len(m.Tasks))
	removed := 0
	for _, t := range m.Tasks {
		if t.Status == "completed" || t.Status == "done" {
			removed++
			continue
		}
		active = append(active, t)
	}
	m.Tasks = active
	return removed
}

// ActiveTasks returns only non-completed tasks
func (m *Manifest) ActiveTasks() []Task {
	var result []Task
	for _, t := range m.Tasks {
		if t.Status != "completed" && t.Status != "done" {
			result = append(result, t)
		}
	}
	return result
}
