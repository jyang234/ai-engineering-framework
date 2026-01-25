package types

import "time"

// Item represents a knowledge item in RECALL
type Item struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"` // pattern, failure, decision, context
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	Tags            []string  `json:"tags,omitempty"`
	Scope           string    `json:"scope"` // global, project
	ProjectPath     string    `json:"project_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	UsefulnessScore float64   `json:"usefulness_score"`
	UseCount        int       `json:"use_count"`
}

// SearchResult represents the result of a RECALL search
type SearchResult struct {
	Results       []Item        `json:"results"`
	Count         int           `json:"count"`
	QueryMetadata QueryMetadata `json:"query_metadata,omitempty"`
}

// QueryMetadata contains information about the search query
type QueryMetadata struct {
	ScopeSearched   string `json:"scope_searched"`
	TotalCandidates int    `json:"total_candidates"`
	Reranked        bool   `json:"reranked"`
	LatencyMs       int    `json:"latency_ms"`
}
