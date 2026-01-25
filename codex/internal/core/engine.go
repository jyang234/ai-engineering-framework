package core

import (
	"context"
	"fmt"

	"github.com/anthropics/aef/codex/internal/embedding"
	"github.com/anthropics/aef/codex/internal/reranking"
	"github.com/anthropics/aef/codex/internal/storage"
)

// SearchEngine orchestrates search and indexing operations
type SearchEngine struct {
	config   Config
	qdrant   *storage.QdrantStorage
	metadata *storage.MetadataStore
	voyage   *embedding.VoyageClient
	openai   *embedding.OpenAIClient
	reranker *reranking.Reranker
}

// NewSearchEngine creates a new search engine instance
func NewSearchEngine(ctx context.Context, config Config) (*SearchEngine, error) {
	// Initialize Qdrant storage
	qdrant, err := storage.NewQdrantStorage(config.QdrantAddr, config.CollectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	// Ensure collection exists with proper config
	if err := qdrant.EnsureCollection(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	// Initialize metadata store
	metadata, err := storage.NewMetadataStore(config.MetadataDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata store: %w", err)
	}

	// Initialize embedding clients
	voyage := embedding.NewVoyageClient(config.VoyageAPIKey)
	openai := embedding.NewOpenAIClient(config.OpenAIAPIKey)

	// Initialize reranker (optional - may fail if models not present)
	var reranker *reranking.Reranker
	if config.ModelsPath != "" {
		reranker, err = reranking.NewReranker(config.ModelsPath)
		if err != nil {
			// Log warning but continue - reranking is optional
			fmt.Printf("Warning: reranker not available: %v\n", err)
		}
	}

	return &SearchEngine{
		config:   config,
		qdrant:   qdrant,
		metadata: metadata,
		voyage:   voyage,
		openai:   openai,
		reranker: reranker,
	}, nil
}

// Close releases all resources
func (e *SearchEngine) Close() error {
	if e.metadata != nil {
		e.metadata.Close()
	}
	if e.reranker != nil {
		e.reranker.Close()
	}
	return nil
}

// Search performs a hybrid search with optional reranking
func (e *SearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Generate query embedding using Voyage (optimized for code queries)
	queryVec, err := e.voyage.EmbedCodeQuery(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Perform hybrid search (vector + BM25)
	candidateLimit := req.Limit
	if e.reranker != nil {
		candidateLimit = 50 // Get more candidates for reranking
	}

	candidates, err := e.qdrant.HybridSearch(ctx, storage.SearchParams{
		Query:       req.Query,
		QueryVector: queryVec,
		Types:       req.Types,
		Scope:       req.Scope,
		Limit:       candidateLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	// Convert to search results
	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SearchResult{
			Item: Item{
				ID:      c.ID,
				Type:    c.Type,
				Title:   c.Title,
				Content: c.Content,
				Tags:    c.Tags,
				Scope:   c.Scope,
			},
			Score: c.Score,
		}
	}

	// Apply reranking if available
	if e.reranker != nil && len(results) > 0 {
		reranked, err := e.reranker.Rerank(req.Query, toDocuments(results), req.Limit)
		if err != nil {
			// Fall back to non-reranked results
			fmt.Printf("Warning: reranking failed: %v\n", err)
		} else {
			results = applyRerankScores(results, reranked)
		}
	}

	// Limit results
	if len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// Get retrieves an item by ID
func (e *SearchEngine) Get(ctx context.Context, id string) (*Item, error) {
	record, err := e.metadata.GetItem(id)
	if err != nil {
		return nil, err
	}
	return itemFromRecord(record), nil
}

// Add adds a new item to the knowledge base
func (e *SearchEngine) Add(ctx context.Context, item *Item) error {
	// Determine embedding type based on item type
	var vec []float32
	var err error

	if item.Type == "pattern" || item.Type == "failure" || item.Type == "code" {
		vec, err = e.voyage.EmbedCode(ctx, []string{item.Content})
		if err != nil {
			return fmt.Errorf("failed to embed code content: %w", err)
		}
	} else {
		vecs, err := e.openai.EmbedDocuments(ctx, []string{item.Content})
		if err != nil {
			return fmt.Errorf("failed to embed document content: %w", err)
		}
		vec = vecs[0]
	}

	// Store in Qdrant
	if err := e.qdrant.Upsert(ctx, item, vec); err != nil {
		return fmt.Errorf("failed to store in Qdrant: %w", err)
	}

	// Store metadata
	if err := e.metadata.SaveItem(itemToRecord(item)); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// RecordFeedback records user feedback on a search result
func (e *SearchEngine) RecordFeedback(feedback *Feedback) error {
	return e.metadata.RecordFeedback(feedbackToRecord(feedback))
}

// LogFlightRecorder logs an entry to the flight recorder
func (e *SearchEngine) LogFlightRecorder(entry *FlightRecorderEntry) error {
	return e.metadata.LogFlightRecorder(entryToRecord(entry))
}

// Helper functions

func toDocuments(results []SearchResult) []reranking.Document {
	docs := make([]reranking.Document, len(results))
	for i, r := range results {
		docs[i] = reranking.Document{
			ID:      r.ID,
			Content: r.Content,
		}
	}
	return docs
}

func applyRerankScores(results []SearchResult, reranked []reranking.RerankResult) []SearchResult {
	// Create ID to result map
	resultMap := make(map[string]*SearchResult)
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	// Reorder based on rerank scores
	newResults := make([]SearchResult, 0, len(reranked))
	for _, r := range reranked {
		if result, ok := resultMap[r.ID]; ok {
			result.Score = r.Score
			newResults = append(newResults, *result)
		}
	}

	return newResults
}

// Type conversion helpers

func itemFromRecord(r *storage.ItemRecord) *Item {
	return &Item{
		ID:        r.ID,
		Type:      r.Type,
		Title:     r.Title,
		Content:   r.Content,
		Tags:      r.Tags,
		Scope:     r.Scope,
		Source:    r.Source,
		Metadata:  r.Metadata,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func itemToRecord(i *Item) *storage.ItemRecord {
	return &storage.ItemRecord{
		ID:        i.ID,
		Type:      i.Type,
		Title:     i.Title,
		Content:   i.Content,
		Tags:      i.Tags,
		Scope:     i.Scope,
		Source:    i.Source,
		Metadata:  i.Metadata,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func feedbackToRecord(f *Feedback) *storage.FeedbackRecord {
	return &storage.FeedbackRecord{
		ID:        storage.GenerateID(),
		ItemID:    f.ItemID,
		SessionID: f.SessionID,
		Useful:    f.Useful,
		Context:   f.Context,
		Timestamp: f.Timestamp,
	}
}

func entryToRecord(e *FlightRecorderEntry) *storage.FlightRecorderRecord {
	return &storage.FlightRecorderRecord{
		ID:        e.ID,
		SessionID: e.SessionID,
		Timestamp: e.Timestamp,
		Type:      e.Type,
		Content:   e.Content,
		Rationale: e.Rationale,
		Metadata:  e.Metadata,
	}
}
