package core

import (
	"time"
)

// Item type constants
const (
	TypePattern  = "pattern"
	TypeFailure  = "failure"
	TypeDecision = "decision"
	TypeContext  = "context"
	TypeCode     = "code"
	TypeDoc      = "doc"
	TypeRunbook  = "runbook"
	TypeManual   = "manual"
)

// Flight recorder entry type constants
const (
	FlightTypeDecision          = "decision"
	FlightTypeError             = "error"
	FlightTypeMilestone         = "milestone"
	FlightTypeObservation       = "observation"
	FlightTypeRetrievalQuery    = "retrieval_query"
	FlightTypeRetrievalJudgment = "retrieval_judgment"
)

// Config holds configuration for the search engine
type Config struct {
	AnthropicAPIKey string
	ModelsPath      string
	MetadataDBPath  string

	// Embedder (nomic-embed-text via Ollama)
	LocalEmbeddingURL   string // e.g. "http://localhost:11434/api/embed"
	LocalEmbeddingModel string // e.g. "nomic-embed-text"

	// ScoreThreshold sets the minimum score as a ratio of the top result's score.
	// Results scoring below topScore * ScoreThreshold are dropped.
	// 0 disables thresholding. Typical value: 0.5
	ScoreThreshold float64
}

// Item represents a knowledge item in Codex
type Item struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`      // pattern, failure, decision, context, code, doc
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags,omitempty"`
	Scope     string            `json:"scope"`     // global, project
	Source    string            `json:"source,omitempty"` // file path or manual
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SearchRequest represents a search query
type SearchRequest struct {
	Query     string   `json:"query"`
	Types     []string `json:"types,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	UseHybrid bool     `json:"use_hybrid,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Item
	Score      float64 `json:"score"`
	Highlights []string `json:"highlights,omitempty"`
}

// IndexRequest represents a request to index content
type IndexRequest struct {
	Content  string   `json:"content"`
	Type     string   `json:"type"`     // code, doc, manual
	FilePath string   `json:"file_path,omitempty"`
	Language string   `json:"language,omitempty"` // for code: go, python, typescript
	Tags     []string `json:"tags,omitempty"`
	Scope    string   `json:"scope,omitempty"`
}

// IndexResult represents the result of an indexing operation
type IndexResult struct {
	ItemID      string `json:"item_id"`
	ChunksCount int    `json:"chunks_count"`
}

// FlightRecorderEntry represents a log entry from the flight recorder
type FlightRecorderEntry struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	Timestamp time.Time      `json:"timestamp"`
	Type      string         `json:"type"` // decision, error, milestone, observation, retrieval_query, retrieval_judgment
	Content   string         `json:"content"`
	Rationale string         `json:"rationale,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Feedback represents user feedback on a search result
type Feedback struct {
	ItemID    string    `json:"item_id"`
	SessionID string    `json:"session_id"`
	Useful    bool      `json:"useful"`
	Context   string    `json:"context,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
