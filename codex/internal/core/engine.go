package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/aef/codex/internal/embedding"
	"github.com/anthropics/aef/codex/internal/reranking"
	"github.com/anthropics/aef/codex/internal/storage"
)

// SearchEngine orchestrates search and indexing operations
type SearchEngine struct {
	config   Config
	vecStore VectorStorage
	metadata MetadataStorage
	keywords KeywordSearcher
	embedder Embedder
	reranker Reranker
}

// SearchEngineDeps holds dependencies for constructing a SearchEngine.
type SearchEngineDeps struct {
	Config   Config
	VecStore VectorStorage
	Metadata MetadataStorage
	Keywords KeywordSearcher
	Embedder Embedder
	Reranker Reranker
}

// NewSearchEngine creates a new search engine with SQLite-backed vector storage.
func NewSearchEngine(ctx context.Context, config Config) (*SearchEngine, error) {
	// Initialize metadata store
	metadata, err := storage.NewMetadataStore(config.MetadataDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata store: %w", err)
	}

	// Initialize vector store sharing the same SQLite database
	vecStore, err := storage.NewVecStore(metadata.DB())
	if err != nil {
		metadata.Close()
		return nil, fmt.Errorf("failed to initialize vector store: %w", err)
	}

	// Initialize embedding client (single local Ollama model)
	var opts []embedding.LocalClientOption
	if config.LocalEmbeddingURL != "" {
		opts = append(opts, embedding.WithLocalBaseURL(config.LocalEmbeddingURL))
	}
	if config.LocalEmbeddingModel != "" {
		opts = append(opts, embedding.WithLocalModel(config.LocalEmbeddingModel))
	}
	embed := embedding.NewLocalClient(opts...)

	// Initialize reranker (optional - may fail if models not present)
	var reranker Reranker
	if config.ModelsPath != "" {
		r, rErr := reranking.NewReranker(config.ModelsPath)
		if rErr != nil {
			log.Printf("Warning: reranker not available: %v\n", rErr)
		} else {
			reranker = r
		}
	}

	return &SearchEngine{
		config:   config,
		vecStore: vecStore,
		metadata: metadata,
		keywords: metadata,
		embedder: embed,
		reranker: reranker,
	}, nil
}

// NewSearchEngineWithDeps creates a search engine with explicit dependencies (for testing).
func NewSearchEngineWithDeps(deps SearchEngineDeps) *SearchEngine {
	return &SearchEngine{
		config:   deps.Config,
		vecStore: deps.VecStore,
		metadata: deps.Metadata,
		keywords: deps.Keywords,
		embedder: deps.Embedder,
		reranker: deps.Reranker,
	}
}

// Close releases all resources
func (e *SearchEngine) Close() error {
	if e.reranker != nil {
		e.reranker.Close()
	}
	if e.metadata != nil {
		return e.metadata.Close()
	}
	return nil
}

// Search performs hybrid search: vector similarity + FTS5 keyword + RRF fusion,
// with optional reranking.
func (e *SearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}

	candidateLimit := 50
	if e.reranker == nil && req.Limit < candidateLimit {
		candidateLimit = req.Limit * 3 // over-fetch for fusion but not too much
		if candidateLimit < 20 {
			candidateLimit = 20
		}
	}

	// 1. Embed query
	queryVec, err := e.embedder.EmbedQuery(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 2. Vector search
	vectorResults, err := e.vecStore.Search(ctx, queryVec, candidateLimit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 3. Keyword search (FTS5 BM25)
	var keywordResults []SearchResult
	if e.keywords != nil {
		kwResults, err := e.keywords.KeywordSearch(req.Query, candidateLimit)
		if err != nil {
			// Log but don't fail -- vector results are still valid
			log.Printf("Warning: keyword search failed: %v\n", err)
		} else {
			for _, kw := range kwResults {
				keywordResults = append(keywordResults, SearchResult{
					Item: Item{
						ID:      kw.ID,
						Type:    kw.Type,
						Title:   kw.Title,
						Content: kw.Content,
						Tags:    kw.Tags,
						Scope:   kw.Scope,
					},
					Score: kw.Score,
				})
			}
		}
	}

	// 4. 2-way RRF fusion (vector + keywords)
	results := reciprocalRankFusion(vectorResults, keywordResults, 60)

	// 5. Hydrate metadata for vector-only results (those missing Title/Content)
	for i := range results {
		if results[i].Title == "" && results[i].Content == "" && e.metadata != nil {
			record, err := e.metadata.GetItem(results[i].ID)
			if err == nil {
				results[i].Item = *itemFromRecord(record)
			}
		}
	}

	// 6. Apply type/scope filters
	if len(req.Types) > 0 || req.Scope != "" {
		typeSet := make(map[string]bool, len(req.Types))
		for _, t := range req.Types {
			typeSet[t] = true
		}
		var filtered []SearchResult
		for _, r := range results {
			if len(typeSet) > 0 && !typeSet[r.Type] {
				continue
			}
			if req.Scope != "" && r.Scope != req.Scope {
				continue
			}
			filtered = append(filtered, r)
		}
		results = filtered
	}

	// 7. Apply reranking if available
	if e.reranker != nil && len(results) > 0 {
		reranked, err := e.reranker.Rerank(req.Query, toDocuments(results), req.Limit)
		if err != nil {
			log.Printf("Warning: reranking failed: %v\n", err)
		} else {
			results = applyRerankScores(results, reranked)
		}
	}

	// 8. Score threshold cutoff — drop results below ratio of top score
	if e.config.ScoreThreshold > 0 && len(results) > 0 {
		minScore := results[0].Score * e.config.ScoreThreshold
		cutoff := len(results)
		for i, r := range results {
			if r.Score < minScore {
				cutoff = i
				break
			}
		}
		results = results[:cutoff]
	}

	// 9. Limit results
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
	vec, err := e.embedder.EmbedDocument(ctx, item.Content)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	// Store metadata first (easier to clean up than orphaned vectors)
	if err := e.metadata.SaveItem(itemToRecord(item)); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Store vector
	if err := e.vecStore.Upsert(ctx, item.ID, vec); err != nil {
		return fmt.Errorf("failed to store vector: %w", err)
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

// GetFlightRecorderEntries retrieves flight recorder entries for a session.
func (e *SearchEngine) GetFlightRecorderEntries(sessionID string) ([]*FlightRecorderEntry, error) {
	records, err := e.metadata.GetFlightRecorderEntries(sessionID)
	if err != nil {
		return nil, err
	}
	entries := make([]*FlightRecorderEntry, len(records))
	for i, r := range records {
		entries[i] = &FlightRecorderEntry{
			ID:        r.ID,
			SessionID: r.SessionID,
			Timestamp: r.Timestamp,
			Type:      r.Type,
			Content:   r.Content,
			Rationale: r.Rationale,
			Metadata:  r.Metadata,
		}
	}
	return entries, nil
}

// Index indexes content through the appropriate pipeline (code, doc, or manual)
func (e *SearchEngine) Index(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	indexer, err := NewIndexer(e)
	if err != nil {
		return nil, err
	}
	defer indexer.Close()

	return indexer.IndexFile(ctx, req)
}

// IndexDirectory indexes all files in a directory
func (e *SearchEngine) IndexDirectory(ctx context.Context, dirPath string, scope string) ([]IndexResult, error) {
	indexer, err := NewIndexer(e)
	if err != nil {
		return nil, err
	}
	defer indexer.Close()

	return indexer.IndexDirectory(ctx, dirPath, scope)
}

// NewIndexer creates an indexer for this engine (for advanced use cases)
func (e *SearchEngine) NewIndexer() (*Indexer, error) {
	return NewIndexer(e)
}

// MigrateFromV0 migrates items from a RECALL v0 SQLite database to this engine
func (e *SearchEngine) MigrateFromV0(ctx context.Context, v0DBPath string) (*MigrationStats, error) {
	return MigrateV0ToV1(ctx, v0DBPath, e)
}

// MigrateFromV0WithProgress migrates with progress callback
func (e *SearchEngine) MigrateFromV0WithProgress(ctx context.Context, v0DBPath string, progress MigrationProgressCallback) (*MigrationStats, error) {
	return MigrateV0ToV1WithProgress(ctx, v0DBPath, e, progress)
}

// List retrieves items with filters and pagination
func (e *SearchEngine) List(ctx context.Context, itemType, scope string, limit, offset int) ([]Item, error) {
	records, err := e.metadata.ListItems(itemType, scope, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]Item, len(records))
	for i, r := range records {
		items[i] = *itemFromRecord(r)
	}

	return items, nil
}

// Update updates an existing item
func (e *SearchEngine) Update(ctx context.Context, item *Item) error {
	// Verify item exists
	_, err := e.metadata.GetItem(item.ID)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}

	// Update timestamp
	item.UpdatedAt = time.Now()

	// Regenerate embedding
	vec, err := e.embedder.EmbedDocument(ctx, item.Content)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	// Update metadata first (matching Add() convention)
	if err := e.metadata.SaveItem(itemToRecord(item)); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Update vector
	if err := e.vecStore.Upsert(ctx, item.ID, vec); err != nil {
		return fmt.Errorf("failed to update vector: %w", err)
	}

	return nil
}

// Delete removes an item from both vector store and metadata store
func (e *SearchEngine) Delete(ctx context.Context, id string) error {
	// Delete from vector store (best-effort — metadata is the source of truth)
	if err := e.vecStore.Delete(ctx, id); err != nil {
		log.Printf("Warning: failed to delete vector for %s: %v", id, err)
	}

	// Delete from metadata store
	if err := e.metadata.DeleteItem(id); err != nil {
		return fmt.Errorf("failed to delete from metadata: %w", err)
	}

	return nil
}

// Stats returns item statistics
func (e *SearchEngine) Stats(ctx context.Context) (map[string]int, error) {
	return e.metadata.CountItemsByType()
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
