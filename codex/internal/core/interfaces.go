package core

import (
	"context"

	"github.com/anthropics/aef/codex/internal/chunking"
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
// Implementations: QdrantStorage
type VectorStorage interface {
	// Upsert adds or updates an item with its embedding vector.
	// item should be *Item but uses any for compatibility with existing storage.
	Upsert(ctx context.Context, item any, vector []float32) error

	// HybridSearch performs combined vector + keyword search.
	HybridSearch(ctx context.Context, params storage.SearchParams) ([]storage.SearchCandidate, error)

	// Delete removes an item by ID.
	Delete(ctx context.Context, id string) error
}

// MetadataStorage stores item metadata and auxiliary data.
// Implementations: MetadataStore (SQLite)
type MetadataStorage interface {
	// SaveItem persists item metadata.
	SaveItem(item *storage.ItemRecord) error

	// GetItem retrieves item metadata by ID.
	GetItem(id string) (*storage.ItemRecord, error)

	// RecordFeedback stores user feedback on search results.
	RecordFeedback(feedback *storage.FeedbackRecord) error

	// LogFlightRecorder logs a flight recorder entry.
	LogFlightRecorder(entry *storage.FlightRecorderRecord) error

	// Close releases storage resources.
	Close() error
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
