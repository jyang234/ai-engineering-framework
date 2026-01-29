package core

import (
	"context"

	"github.com/anthropics/aef/codex/internal/chunking"
	"github.com/anthropics/aef/codex/internal/reranking"
	"github.com/anthropics/aef/codex/internal/storage"
)

// CodeEmbedder generates embeddings for code content.
// Implementations: VoyageClient
type CodeEmbedder interface {
	// EmbedCode embeds code snippets for storage/indexing.
	// Returns a single embedding vector for the first text.
	EmbedCode(ctx context.Context, texts []string) ([]float32, error)

	// EmbedCodeQuery embeds a search query (may use different model settings).
	EmbedCodeQuery(ctx context.Context, query string) ([]float32, error)
}

// DocEmbedder generates embeddings for document content.
// Implementations: OpenAIClient
type DocEmbedder interface {
	// EmbedDocuments embeds document texts for storage/indexing.
	// Returns one embedding per input text.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorStorage stores and searches vector embeddings.
// Implementations: VecStore (SQLite + brute-force KNN)
type VectorStorage interface {
	// Upsert stores a vector for the given item ID.
	Upsert(ctx context.Context, itemID string, vector []float32) error

	// Search returns the top-K items by cosine similarity.
	Search(ctx context.Context, queryVec []float32, limit int) []storage.ScoredResult

	// Delete removes an item by ID.
	Delete(ctx context.Context, itemID string) error
}

// KeywordSearcher performs full-text keyword search.
// Implementations: MetadataStore (FTS5)
type KeywordSearcher interface {
	KeywordSearch(query string, limit int) ([]storage.KeywordResult, error)
}

// MetadataStorage stores item metadata and auxiliary data.
// Implementations: MetadataStore (SQLite)
type MetadataStorage interface {
	SaveItem(item *storage.ItemRecord) error
	GetItem(id string) (*storage.ItemRecord, error)
	ListItems(itemType, scope string, limit, offset int) ([]*storage.ItemRecord, error)
	DeleteItem(id string) error
	CountItemsByType() (map[string]int, error)
	RecordFeedback(feedback *storage.FeedbackRecord) error
	LogFlightRecorder(entry *storage.FlightRecorderRecord) error
	GetFlightRecorderEntries(sessionID string) ([]*storage.FlightRecorderRecord, error)
	Close() error
}

// Reranker reorders search results using a cross-encoder model.
// Implementations: reranking.Reranker (BGE/ONNX)
type Reranker interface {
	Rerank(query string, docs []reranking.Document, topK int) ([]reranking.RerankResult, error)
	Close()
}

// CodeChunker splits code into semantic chunks.
// Implementations: ASTChunker
type CodeChunker interface {
	// ChunkFile extracts semantic chunks from source code.
	// lang is the programming language (go, python, typescript, etc.)
	ChunkFile(content []byte, lang, filePath string) ([]chunking.CodeChunk, error)

	// Close releases chunker resources.
	Close() error
}

// DocChunker splits documents into chunks with optional enrichment.
// Implementations: ContextualChunker
type DocChunker interface {
	// ChunkDocument splits a document and optionally enriches chunks.
	ChunkDocument(ctx context.Context, content, filePath string) ([]chunking.DocChunk, error)
}

// IDGenerator generates unique identifiers.
// Implementations: storage.GenerateID (UUID-based)
type IDGenerator interface {
	GenerateID() string
}

// defaultIDGenerator uses UUID for ID generation
type defaultIDGenerator struct{}

func (g *defaultIDGenerator) GenerateID() string {
	return storage.GenerateID()
}

// NewIDGenerator creates a default ID generator.
func NewIDGenerator() IDGenerator {
	return &defaultIDGenerator{}
}
