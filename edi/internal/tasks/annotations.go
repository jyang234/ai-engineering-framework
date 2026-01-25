package tasks

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// TaskAnnotation stores RECALL context for a task
type TaskAnnotation struct {
	TaskID      string    `yaml:"task_id"`
	Description string    `yaml:"description"`
	CreatedAt   time.Time `yaml:"created_at"`

	RecallContext RecallContext `yaml:"recall_context"`

	InheritedContext []InheritedDecision `yaml:"inherited_context,omitempty"`

	ExecutionContext ExecutionContext `yaml:"execution_context,omitempty"`
}

// RecallContext holds RECALL items relevant to a task
type RecallContext struct {
	Patterns  []string `yaml:"patterns,omitempty"`
	Failures  []string `yaml:"failures,omitempty"`
	Decisions []string `yaml:"decisions,omitempty"`
	Query     string   `yaml:"query,omitempty"`
}

// InheritedDecision represents a decision inherited from a parent task
type InheritedDecision struct {
	FromTaskID string `yaml:"from_task_id"`
	Decision   string `yaml:"decision"`
	Rationale  string `yaml:"rationale,omitempty"`
}

// ExecutionContext captures what happened during task execution
type ExecutionContext struct {
	DecisionsMade []Decision `yaml:"decisions_made,omitempty"`
	Discoveries   []string   `yaml:"discoveries,omitempty"`
	CapturedTo    []string   `yaml:"captured_to,omitempty"` // RECALL IDs
}

// Decision represents a decision made during task execution
type Decision struct {
	Summary   string `yaml:"summary"`
	Rationale string `yaml:"rationale,omitempty"`
	Propagate bool   `yaml:"propagate"`
}

// LoadAnnotation loads a task annotation from disk
func LoadAnnotation(projectPath, taskID string) (*TaskAnnotation, error) {
	path := filepath.Join(projectPath, ".edi", "tasks", taskID+".yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var annotation TaskAnnotation
	if err := yaml.Unmarshal(data, &annotation); err != nil {
		return nil, err
	}

	return &annotation, nil
}

// SaveAnnotation saves a task annotation to disk
func SaveAnnotation(projectPath string, annotation *TaskAnnotation) error {
	dir := filepath.Join(projectPath, ".edi", "tasks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, annotation.TaskID+".yaml")

	data, err := yaml.Marshal(annotation)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ListAnnotations lists all task annotations in a project
func ListAnnotations(projectPath string) ([]*TaskAnnotation, error) {
	dir := filepath.Join(projectPath, ".edi", "tasks")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var annotations []*TaskAnnotation
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		taskID := entry.Name()[:len(entry.Name())-5] // Remove .yaml
		annotation, err := LoadAnnotation(projectPath, taskID)
		if err != nil {
			continue
		}

		annotations = append(annotations, annotation)
	}

	return annotations, nil
}
