package recall

import "time"

// Item represents a knowledge item in the RECALL system.
type Item struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Tags      []string       `json:"tags,omitempty"`
	Scope     string         `json:"scope"`
	Source    string         `json:"source,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// SearchOpts configures a search query.
type SearchOpts struct {
	Types []string
	Scope string
	Limit int
}

// Config configures a recall Client.
type Config struct {
	MetadataDBPath string

	// Optional — degrades gracefully to FTS-only if embedding unavailable
	LocalEmbeddingURL   string
	LocalEmbeddingModel string
}
