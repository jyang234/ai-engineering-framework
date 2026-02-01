package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ShortID safely returns up to the first 8 characters of an ID string.
func ShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// SaveHistory saves a session history entry
func SaveHistory(projectPath string, entry *HistoryEntry) error {
	historyDir := filepath.Join(projectPath, ".edi", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Generate filename: {date}-{session-id}.md
	filename := fmt.Sprintf("%s-%s.md",
		entry.Date.Format("2006-01-02"),
		ShortID(entry.SessionID))

	path := filepath.Join(historyDir, filename)

	// Write frontmatter + body
	content, err := formatHistoryEntry(entry)
	if err != nil {
		return fmt.Errorf("failed to format history entry: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

func formatHistoryEntry(entry *HistoryEntry) (string, error) {
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

	frontmatterYAML, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	return fmt.Sprintf("---\n%s---\n\n%s", string(frontmatterYAML), entry.Summary), nil
}

// FlightRecorderFile manages flight recorder JSONL output
type FlightRecorderFile struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// NewFlightRecorderFile creates a new flight recorder file writer
func NewFlightRecorderFile(projectPath, sessionID string) (*FlightRecorderFile, error) {
	historyDir := filepath.Join(projectPath, ".edi", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	filename := fmt.Sprintf("%s-flight.jsonl", ShortID(sessionID))
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
	f.mu.Lock()
	defer f.mu.Unlock()
	// Copy to avoid aliasing the caller's slice via append
	buf := make([]byte, len(entry)+1)
	copy(buf, entry)
	buf[len(entry)] = '\n'
	_, err := f.file.Write(buf)
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
