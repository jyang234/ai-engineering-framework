package briefing

import (
	"os"

	"github.com/anthropics/aef/edi/internal/tasks"
)

// TaskStatus represents the status of tasks
type TaskStatus struct {
	Total           int
	Completed       int
	InProgress      int
	Pending         int
	InProgressItems []TaskItem
	ReadyItems      []TaskItem
}

// TaskItem represents a single task
type TaskItem struct {
	ID          string   `json:"id"`
	Description string   `json:"subject"`
	Status      string   `json:"status"`
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blockedBy"`
}

func loadTaskStatus(projectPath string) (*TaskStatus, error) {
	status := &TaskStatus{}

	// Check if this is an EDI project
	ediDir := projectPath + "/.edi"
	if _, err := os.Stat(ediDir); os.IsNotExist(err) {
		return status, nil // Not an EDI project
	}

	// Load tasks from manifest
	manifest, err := tasks.LoadManifest(projectPath)
	if err != nil {
		return status, nil // Error loading manifest, return empty status
	}

	for _, task := range manifest.Tasks {
		status.Total++

		item := TaskItem{
			ID:          task.ID,
			Description: task.Subject,
			Status:      task.Status,
			Blocks:      task.Blocks,
			BlockedBy:   task.BlockedBy,
		}

		switch task.Status {
		case "completed", "done":
			status.Completed++
		case "in_progress", "active":
			status.InProgress++
			status.InProgressItems = append(status.InProgressItems, item)
		default:
			status.Pending++
			if len(task.BlockedBy) == 0 {
				status.ReadyItems = append(status.ReadyItems, item)
			}
		}
	}

	return status, nil
}

