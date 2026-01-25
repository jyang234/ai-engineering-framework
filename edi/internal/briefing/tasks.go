package briefing

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	home, _ := os.UserHomeDir()
	tasksDir := filepath.Join(home, ".claude", "tasks")

	status := &TaskStatus{}

	// Find task list directories
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return status, nil // No tasks is fine
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionDir := filepath.Join(tasksDir, entry.Name())
		taskFiles, _ := os.ReadDir(sessionDir)

		for _, tf := range taskFiles {
			if filepath.Ext(tf.Name()) != ".json" {
				continue
			}

			taskPath := filepath.Join(sessionDir, tf.Name())
			task, err := loadTask(taskPath)
			if err != nil {
				continue
			}

			status.Total++

			switch task.Status {
			case "completed", "done":
				status.Completed++
			case "in_progress", "active":
				status.InProgress++
				status.InProgressItems = append(status.InProgressItems, task)
			default:
				status.Pending++
				if len(task.BlockedBy) == 0 {
					status.ReadyItems = append(status.ReadyItems, task)
				}
			}
		}
	}

	return status, nil
}

func loadTask(path string) (TaskItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TaskItem{}, err
	}

	var task TaskItem
	if err := json.Unmarshal(data, &task); err != nil {
		return TaskItem{}, err
	}

	return task, nil
}
