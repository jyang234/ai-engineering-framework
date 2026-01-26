package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/codex/internal/chunking"
)

// Indexer handles content indexing through the appropriate pipeline
type Indexer struct {
	// Dependencies (interfaces for testability)
	codeEmbedder CodeEmbedder
	docEmbedder  DocEmbedder
	vectorStore  VectorStorage
	metaStore    MetadataStorage
	codeChunker  CodeChunker
	docChunker   DocChunker // optional
	idGen        IDGenerator
}

// IndexerConfig holds configuration for creating an Indexer
type IndexerConfig struct {
	CodeEmbedder CodeEmbedder
	DocEmbedder  DocEmbedder
	VectorStore  VectorStorage
	MetaStore    MetadataStorage
	CodeChunker  CodeChunker
	DocChunker   DocChunker // optional - for contextual enrichment
	IDGenerator  IDGenerator
}

// NewIndexer creates a new indexer from a SearchEngine (convenience constructor)
func NewIndexer(engine *SearchEngine) (*Indexer, error) {
	astChunker := chunking.NewASTChunker()

	// Contextual chunker is optional - requires Anthropic API key
	var ctxChunker DocChunker
	if engine.config.AnthropicAPIKey != "" {
		chunker, err := chunking.NewContextualChunker(engine.config.AnthropicAPIKey)
		if err != nil {
			// Log but continue - contextual enrichment is optional
			fmt.Printf("Warning: contextual chunker not available: %v\n", err)
		} else {
			ctxChunker = chunker
		}
	}

	return &Indexer{
		codeEmbedder: engine.voyage,
		docEmbedder:  engine.openai,
		vectorStore:  engine.qdrant,
		metaStore:    engine.metadata,
		codeChunker:  astChunker,
		docChunker:   ctxChunker,
		idGen:        NewIDGenerator(),
	}, nil
}

// NewIndexerWithConfig creates an Indexer with explicit dependencies (for testing)
func NewIndexerWithConfig(cfg IndexerConfig) (*Indexer, error) {
	if cfg.CodeEmbedder == nil {
		return nil, fmt.Errorf("CodeEmbedder is required")
	}
	if cfg.DocEmbedder == nil {
		return nil, fmt.Errorf("DocEmbedder is required")
	}
	if cfg.VectorStore == nil {
		return nil, fmt.Errorf("VectorStore is required")
	}
	if cfg.MetaStore == nil {
		return nil, fmt.Errorf("MetaStore is required")
	}
	if cfg.CodeChunker == nil {
		return nil, fmt.Errorf("CodeChunker is required")
	}

	idGen := cfg.IDGenerator
	if idGen == nil {
		idGen = NewIDGenerator()
	}

	return &Indexer{
		codeEmbedder: cfg.CodeEmbedder,
		docEmbedder:  cfg.DocEmbedder,
		vectorStore:  cfg.VectorStore,
		metaStore:    cfg.MetaStore,
		codeChunker:  cfg.CodeChunker,
		docChunker:   cfg.DocChunker,
		idGen:        idGen,
	}, nil
}

// IndexFile indexes a file, routing through the appropriate pipeline
func (idx *Indexer) IndexFile(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	// Detect content type and route appropriately
	if req.Type == "" {
		req.Type = detectContentType(req.FilePath, req.Content)
	}

	switch req.Type {
	case "code":
		return idx.indexCode(ctx, req)
	case "doc":
		return idx.indexDoc(ctx, req)
	default:
		return idx.indexManual(ctx, req)
	}
}

// IndexDirectory recursively indexes all files in a directory
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string, scope string) ([]IndexResult, error) {
	var results []IndexResult

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-indexable files
		if !isIndexable(path) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: failed to read %s: %v\n", path, err)
			return nil
		}

		// Index the file
		result, err := idx.IndexFile(ctx, IndexRequest{
			Content:  string(content),
			FilePath: path,
			Scope:    scope,
		})
		if err != nil {
			fmt.Printf("Warning: failed to index %s: %v\n", path, err)
			return nil
		}

		results = append(results, *result)
		return nil
	})

	return results, err
}

// indexCode processes code files through AST chunking
func (idx *Indexer) indexCode(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	// Detect language if not specified
	lang := req.Language
	if lang == "" {
		lang = chunking.DetectLanguage(req.FilePath)
	}

	// Chunk the code using AST parser
	chunks, err := idx.codeChunker.ChunkFile([]byte(req.Content), lang, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("AST chunking failed: %w", err)
	}

	// Generate a parent item ID
	parentID := idx.idGen.GenerateID()
	now := time.Now()

	// Process each chunk
	for i, chunk := range chunks {
		// Generate embedding using code embedder
		vec, err := idx.codeEmbedder.EmbedCode(ctx, []string{chunk.Content})
		if err != nil {
			return nil, fmt.Errorf("failed to embed chunk %d: %w", i, err)
		}

		// Create item for this chunk
		item := &Item{
			ID:      fmt.Sprintf("%s-chunk-%d", parentID, i),
			Type:    "code",
			Title:   buildCodeTitle(chunk),
			Content: chunk.Content,
			Tags:    req.Tags,
			Scope:   req.Scope,
			Source:  req.FilePath,
			Metadata: map[string]any{
				"parent_id":  parentID,
				"chunk_type": chunk.Type,
				"name":       chunk.Name,
				"signature":  chunk.Signature,
				"start_line": chunk.StartLine,
				"end_line":   chunk.EndLine,
				"language":   chunk.Language,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Store in vector storage
		if err := idx.vectorStore.Upsert(ctx, item, vec); err != nil {
			return nil, fmt.Errorf("failed to store chunk %d: %w", i, err)
		}

		// Store metadata
		if err := idx.metaStore.SaveItem(itemToRecord(item)); err != nil {
			return nil, fmt.Errorf("failed to save chunk %d metadata: %w", i, err)
		}
	}

	return &IndexResult{
		ItemID:      parentID,
		ChunksCount: len(chunks),
	}, nil
}

// indexDoc processes documentation through markdown chunking
func (idx *Indexer) indexDoc(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	var chunks []docChunkData

	// Use contextual chunker if available, otherwise basic markdown chunking
	if idx.docChunker != nil {
		docChunks, err := idx.docChunker.ChunkDocument(ctx, req.Content, req.FilePath)
		if err != nil {
			// Fall back to basic chunking on error
			fmt.Printf("Warning: contextual chunking failed, using basic: %v\n", err)
			chunks = basicDocChunking(req.Content, req.FilePath)
		} else {
			for _, dc := range docChunks {
				chunks = append(chunks, docChunkData{
					content:   dc.EnrichedContent,
					section:   dc.Section,
					startLine: dc.StartLine,
					endLine:   dc.EndLine,
				})
			}
		}
	} else {
		chunks = basicDocChunking(req.Content, req.FilePath)
	}

	// Generate a parent item ID
	parentID := idx.idGen.GenerateID()
	now := time.Now()

	// Process each chunk
	for i, chunk := range chunks {
		// Generate embedding using doc embedder
		vecs, err := idx.docEmbedder.EmbedDocuments(ctx, []string{chunk.content})
		if err != nil {
			return nil, fmt.Errorf("failed to embed doc chunk %d: %w", i, err)
		}

		// Create item for this chunk
		item := &Item{
			ID:      fmt.Sprintf("%s-chunk-%d", parentID, i),
			Type:    "doc",
			Title:   chunk.section,
			Content: chunk.content,
			Tags:    req.Tags,
			Scope:   req.Scope,
			Source:  req.FilePath,
			Metadata: map[string]any{
				"parent_id":  parentID,
				"section":    chunk.section,
				"start_line": chunk.startLine,
				"end_line":   chunk.endLine,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Store in vector storage
		if err := idx.vectorStore.Upsert(ctx, item, vecs[0]); err != nil {
			return nil, fmt.Errorf("failed to store doc chunk %d: %w", i, err)
		}

		// Store metadata
		if err := idx.metaStore.SaveItem(itemToRecord(item)); err != nil {
			return nil, fmt.Errorf("failed to save doc chunk %d metadata: %w", i, err)
		}
	}

	return &IndexResult{
		ItemID:      parentID,
		ChunksCount: len(chunks),
	}, nil
}

// indexManual processes manually added items (patterns, failures, decisions, etc.)
func (idx *Indexer) indexManual(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	now := time.Now()
	itemID := idx.idGen.GenerateID()

	// Determine embedding model based on item type
	var vec []float32
	var err error

	itemType := req.Type
	if itemType == "" {
		itemType = "context" // default type
	}

	// Code-related items use code embedder, others use doc embedder
	if itemType == "pattern" || itemType == "failure" {
		vec, err = idx.codeEmbedder.EmbedCode(ctx, []string{req.Content})
		if err != nil {
			return nil, fmt.Errorf("failed to embed: %w", err)
		}
	} else {
		vecs, embedErr := idx.docEmbedder.EmbedDocuments(ctx, []string{req.Content})
		if embedErr != nil {
			return nil, fmt.Errorf("failed to embed: %w", embedErr)
		}
		vec = vecs[0]
	}

	// Extract title from content if not provided
	title := extractTitle(req.Content)

	item := &Item{
		ID:        itemID,
		Type:      itemType,
		Title:     title,
		Content:   req.Content,
		Tags:      req.Tags,
		Scope:     req.Scope,
		Source:    "manual",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Store in vector storage
	if err = idx.vectorStore.Upsert(ctx, item, vec); err != nil {
		return nil, fmt.Errorf("failed to store item: %w", err)
	}

	// Store metadata
	if err = idx.metaStore.SaveItem(itemToRecord(item)); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return &IndexResult{
		ItemID:      itemID,
		ChunksCount: 1,
	}, nil
}

// Close releases indexer resources
func (idx *Indexer) Close() error {
	if idx.codeChunker != nil {
		return idx.codeChunker.Close()
	}
	return nil
}

// Helper types and functions

type docChunkData struct {
	content   string
	section   string
	startLine int
	endLine   int
}

func detectContentType(filePath, content string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Code files
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".jsx": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".cs": true,
	}
	if codeExts[ext] {
		return "code"
	}

	// Documentation files
	docExts := map[string]bool{
		".md": true, ".mdx": true, ".txt": true, ".rst": true,
		".adoc": true, ".org": true,
	}
	if docExts[ext] {
		return "doc"
	}

	// Default to manual/generic
	return "manual"
}

func isIndexable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	indexableExts := map[string]bool{
		// Code
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".jsx": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".cs": true,
		// Docs
		".md": true, ".mdx": true, ".txt": true, ".rst": true,
	}

	return indexableExts[ext]
}

func buildCodeTitle(chunk chunking.CodeChunk) string {
	if chunk.Name != "" {
		return fmt.Sprintf("%s %s in %s", chunk.Type, chunk.Name, filepath.Base(chunk.FilePath))
	}
	return fmt.Sprintf("%s chunk in %s:%d-%d", chunk.Type, filepath.Base(chunk.FilePath), chunk.StartLine, chunk.EndLine)
}

func basicDocChunking(content, filePath string) []docChunkData {
	sections := chunking.ChunkMarkdown(content, 2000) // ~2000 char chunks

	var chunks []docChunkData
	for _, section := range sections {
		chunks = append(chunks, docChunkData{
			content:   section.Content,
			section:   section.Title,
			startLine: section.StartLine,
			endLine:   section.EndLine,
		})
	}

	return chunks
}

func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Truncate if too long
			if len(line) > 100 {
				return line[:97] + "..."
			}
			return line
		}
	}
	return "(Untitled)"
}
