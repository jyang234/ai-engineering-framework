package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// SaveHistory saves a session history entry
func SaveHistory(projectPath string, entry *HistoryEntry) error {
	historyDir := filepath.Join(projectPath, ".edi", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Generate filename: {date}-{session-id}.md
	filename := fmt.Sprintf("%s-%s.md",
		entry.Date.Format("2006-01-02"),
		entry.SessionID[:8])

	path := filepath.Join(historyDir, filename)

	// Write frontmatter + body
	content := formatHistoryEntry(entry)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

func formatHistoryEntry(entry *HistoryEntry) string {
	// Build frontmatter
	frontmatter := struct {
		SessionID         string    `yaml:"session_id"`
		StartedAt         time.Time `yaml:"started_at"`
		EndedAt           time.Time `yaml:"ended_at"`
		Agent             string    `yaml:"agent"`
		TasksCompleted    []string  `yaml:"tasks_completed,omitempty"`
		DecisionsCaptured []string  `yaml:"decisions_captured,omitempty"`
	}{
		SessionID:         entry.SessionID,
		StartedAt:         entry.Date,
		EndedAt:           entry.EndedAt,
		Agent:             entry.Agent,
		TasksCompleted:    entry.TasksCompleted,
		DecisionsCaptured: entry.DecisionsCaptured,
	}

	frontmatterYAML, _ := yaml.Marshal(frontmatter)

	return fmt.Sprintf("---\n%s---\n\n%s", string(frontmatterYAML), entry.Summary)
}

// FlightRecorderFile manages flight recorder JSONL output
type FlightRecorderFile struct {
	path string
	file *os.File
}

// NewFlightRecorderFile creates a new flight recorder file writer
func NewFlightRecorderFile(projectPath, sessionID string) (*FlightRecorderFile, error) {
	historyDir := filepath.Join(projectPath, ".edi", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	filename := fmt.Sprintf("%s-flight.jsonl", sessionID[:8])
	path := filepath.Join(historyDir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open flight recorder file: %w", err)
	}

	return &FlightRecorderFile{
		path: path,
		file: file,
	}, nil
}

// Write writes an entry to the flight recorder file
func (f *FlightRecorderFile) Write(entry []byte) error {
	_, err := f.file.Write(append(entry, '\n'))
	return err
}

// Close closes the flight recorder file
func (f *FlightRecorderFile) Close() error {
	return f.file.Close()
}

// Path returns the path to the flight recorder file
func (f *FlightRecorderFile) Path() string {
	return f.path
}
