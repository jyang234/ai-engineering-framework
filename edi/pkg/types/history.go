package types

import "time"

// HistoryEntry represents a session history entry
type HistoryEntry struct {
	SessionID         string    `yaml:"session_id" json:"session_id"`
	StartedAt         time.Time `yaml:"started_at" json:"started_at"`
	EndedAt           time.Time `yaml:"ended_at" json:"ended_at"`
	Agent             string    `yaml:"agent" json:"agent"`
	TasksCompleted    []string  `yaml:"tasks_completed" json:"tasks_completed"`
	DecisionsCaptured []string  `yaml:"decisions_captured" json:"decisions_captured"`
	Summary           string    `yaml:"-" json:"summary,omitempty"`
}

// FlightRecorderEntry represents a flight recorder log entry
type FlightRecorderEntry struct {
	ID        int64                  `json:"id,omitempty"`
	SessionID string                 `json:"session_id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // decision, error, milestone, observation, task_annotation, task_complete
	Content   string                 `json:"content"`
	Rationale string                 `json:"rationale,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
