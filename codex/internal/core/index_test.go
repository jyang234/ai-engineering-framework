package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/anthropics/aef/codex/internal/chunking"
)

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  string
		want     string
	}{
		// Code files
		{"Go file", "main.go", "", "code"},
		{"Python file", "script.py", "", "code"},
		{"TypeScript file", "app.ts", "", "code"},
		{"TSX file", "component.tsx", "", "code"},
		{"JavaScript file", "index.js", "", "code"},
		{"JSX file", "App.jsx", "", "code"},
		{"Rust file", "lib.rs", "", "code"},
		{"Java file", "Main.java", "", "code"},
		{"C file", "main.c", "", "code"},
		{"C++ file", "main.cpp", "", "code"},
		{"Header file", "types.h", "", "code"},

		// Doc files
		{"Markdown file", "README.md", "", "doc"},
		{"MDX file", "page.mdx", "", "doc"},
		{"Text file", "notes.txt", "", "doc"},
		{"RST file", "docs.rst", "", "doc"},

		// Unknown/manual
		{"JSON file", "config.json", "", "manual"},
		{"YAML file", "config.yaml", "", "manual"},
		{"No extension", "Makefile", "", "manual"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.filePath, tt.content)
			if got != tt.want {
				t.Errorf("detectContentType(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestIsIndexable(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Indexable
		{"main.go", true},
		{"script.py", true},
		{"app.ts", true},
		{"README.md", true},
		{"docs.txt", true},

		// Not indexable
		{"config.json", false},
		{"config.yaml", false},
		{"image.png", false},
		{"binary.exe", false},
		{".gitignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isIndexable(tt.path)
			if got != tt.want {
				t.Errorf("isIndexable(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildCodeTitle(t *testing.T) {
	tests := []struct {
		name  string
		chunk chunking.CodeChunk
		want  string
	}{
		{
			name: "Function with name",
			chunk: chunking.CodeChunk{
				Type:     "function",
				Name:     "ProcessData",
				FilePath: "/path/to/handler.go",
			},
			want: "function ProcessData in handler.go",
		},
		{
			name: "Method with name",
			chunk: chunking.CodeChunk{
				Type:     "method",
				Name:     "HandleRequest",
				FilePath: "/path/to/server.go",
			},
			want: "method HandleRequest in server.go",
		},
		{
			name: "Chunk without name",
			chunk: chunking.CodeChunk{
				Type:      "chunk",
				Name:      "",
				FilePath:  "/path/to/utils.go",
				StartLine: 10,
				EndLine:   50,
			},
			want: "chunk chunk in utils.go:10-50",
		},
		{
			name: "Class with name",
			chunk: chunking.CodeChunk{
				Type:     "class",
				Name:     "UserService",
				FilePath: "/src/services/user.py",
			},
			want: "class UserService in user.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCodeTitle(tt.chunk)
			if got != tt.want {
				t.Errorf("buildCodeTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Simple content",
			content: "This is the title\nThis is the body",
			want:    "This is the title",
		},
		{
			name:    "Content with leading newlines",
			content: "\n\n\nActual title here\nBody content",
			want:    "Actual title here",
		},
		{
			name:    "Empty content",
			content: "",
			want:    "(Untitled)",
		},
		{
			name:    "Only whitespace",
			content: "   \n\n   \n",
			want:    "(Untitled)",
		},
		{
			name:    "Long title gets truncated",
			content: "This is a very long title that exceeds one hundred characters and should be truncated to fit within the limit we have set for titles in our system",
			want:    "This is a very long title that exceeds one hundred characters and should be truncated to fit with...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBasicDocChunking(t *testing.T) {
	content := `# Introduction

This is the introduction paragraph.

# Features

## Feature One

Description of feature one.

## Feature Two

Description of feature two.

# Conclusion

Final thoughts here.`

	chunks := basicDocChunking(content, "/docs/readme.md")

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Check that we got multiple sections
	if len(chunks) < 3 {
		t.Errorf("Expected at least 3 chunks for multi-section doc, got %d", len(chunks))
	}

	// Check first chunk has a section name
	if chunks[0].section == "" {
		t.Error("Expected first chunk to have a section name")
	}
}

func TestDocChunkDataFields(t *testing.T) {
	chunks := basicDocChunking("# Test\n\nContent here", "/test.md")

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	chunk := chunks[0]

	if chunk.content == "" {
		t.Error("Expected chunk to have content")
	}

	if chunk.startLine < 1 {
		t.Errorf("Expected startLine >= 1, got %d", chunk.startLine)
	}

	if chunk.endLine < chunk.startLine {
		t.Errorf("Expected endLine >= startLine, got startLine=%d, endLine=%d",
			chunk.startLine, chunk.endLine)
	}
}

// TestIndexerCreation tests that we can create an indexer with mocks
func TestNewIndexerWithConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		}

		idx, err := NewIndexerWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewIndexerWithConfig failed: %v", err)
		}
		if idx == nil {
			t.Fatal("expected non-nil indexer")
		}
	})

	t.Run("missing CodeEmbedder", func(t *testing.T) {
		cfg := IndexerConfig{
			DocEmbedder: NewMockDocEmbedder(),
			VectorStore: NewMockVectorStorage(),
			MetaStore:   NewMockMetadataStorage(),
			CodeChunker: NewMockCodeChunker(),
		}

		_, err := NewIndexerWithConfig(cfg)
		if err == nil {
			t.Fatal("expected error for missing CodeEmbedder")
		}
	})

	t.Run("missing DocEmbedder", func(t *testing.T) {
		cfg := IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		}

		_, err := NewIndexerWithConfig(cfg)
		if err == nil {
			t.Fatal("expected error for missing DocEmbedder")
		}
	})

	t.Run("missing VectorStore", func(t *testing.T) {
		cfg := IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  NewMockDocEmbedder(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		}

		_, err := NewIndexerWithConfig(cfg)
		if err == nil {
			t.Fatal("expected error for missing VectorStore")
		}
	})
}

// TestIndexer_IndexCode tests code indexing with mocks
func TestIndexer_IndexCode(t *testing.T) {
	ctx := context.Background()

	t.Run("successful indexing", func(t *testing.T) {
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()
		codeChunker := NewMockCodeChunker()

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: codeEmbed,
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  vectorStore,
			MetaStore:    metaStore,
			CodeChunker:  codeChunker,
			IDGenerator:  NewMockIDGenerator("test"),
		})

		result, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "func main() { fmt.Println(\"hello\") }",
			Type:     "code",
			FilePath: "main.go",
			Language: "go",
			Scope:    "project",
		})

		if err != nil {
			t.Fatalf("IndexFile failed: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.ChunksCount != 1 {
			t.Errorf("expected 1 chunk, got %d", result.ChunksCount)
		}
		if codeEmbed.CallCount != 1 {
			t.Errorf("expected 1 embed call, got %d", codeEmbed.CallCount)
		}
		if vectorStore.UpsertCount != 1 {
			t.Errorf("expected 1 upsert, got %d", vectorStore.UpsertCount)
		}
		if metaStore.SaveCount != 1 {
			t.Errorf("expected 1 save, got %d", metaStore.SaveCount)
		}
	})

	t.Run("embedding failure", func(t *testing.T) {
		codeEmbed := NewMockCodeEmbedder()
		codeEmbed.FailOnCall = 1 // Fail on first call

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: codeEmbed,
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "func main() {}",
			Type:     "code",
			FilePath: "main.go",
		})

		if err == nil {
			t.Fatal("expected error on embedding failure")
		}
		if !errors.Is(err, ErrMockEmbedding) {
			t.Errorf("expected ErrMockEmbedding, got %v", err)
		}
	})

	t.Run("vector storage failure", func(t *testing.T) {
		vectorStore := NewMockVectorStorage()
		vectorStore.FailOnUpsert = 1

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  vectorStore,
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "func main() {}",
			Type:     "code",
			FilePath: "main.go",
		})

		if err == nil {
			t.Fatal("expected error on storage failure")
		}
		if !errors.Is(err, ErrMockStorage) {
			t.Errorf("expected ErrMockStorage, got %v", err)
		}
	})

	t.Run("metadata storage failure", func(t *testing.T) {
		metaStore := NewMockMetadataStorage()
		metaStore.FailOnSave = 1

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    metaStore,
			CodeChunker:  NewMockCodeChunker(),
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "func main() {}",
			Type:     "code",
			FilePath: "main.go",
		})

		if err == nil {
			t.Fatal("expected error on metadata save failure")
		}
		if !errors.Is(err, ErrMockStorage) {
			t.Errorf("expected ErrMockStorage, got %v", err)
		}
	})

	t.Run("chunking failure", func(t *testing.T) {
		codeChunker := NewMockCodeChunker()
		codeChunker.FailOnCall = 1

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  codeChunker,
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "func main() {}",
			Type:     "code",
			FilePath: "main.go",
		})

		if err == nil {
			t.Fatal("expected error on chunking failure")
		}
		if !errors.Is(err, ErrMockChunking) {
			t.Errorf("expected ErrMockChunking, got %v", err)
		}
	})
}

// TestIndexer_IndexDoc tests document indexing with mocks
func TestIndexer_IndexDoc(t *testing.T) {
	ctx := context.Background()

	t.Run("successful indexing without contextual chunker", func(t *testing.T) {
		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  docEmbed,
			VectorStore:  vectorStore,
			MetaStore:    metaStore,
			CodeChunker:  NewMockCodeChunker(),
			// No DocChunker - will use basic markdown chunking
		})

		result, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "# Title\n\nSome content here.",
			Type:     "doc",
			FilePath: "README.md",
			Scope:    "project",
		})

		if err != nil {
			t.Fatalf("IndexFile failed: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if docEmbed.CallCount < 1 {
			t.Error("expected at least 1 embed call")
		}
	})

	t.Run("successful indexing with contextual chunker", func(t *testing.T) {
		docEmbed := NewMockDocEmbedder()
		docChunker := NewMockDocChunker()

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  docEmbed,
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
			DocChunker:   docChunker,
		})

		result, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "# Title\n\nSome content here.",
			Type:     "doc",
			FilePath: "README.md",
		})

		if err != nil {
			t.Fatalf("IndexFile failed: %v", err)
		}
		if docChunker.CallCount != 1 {
			t.Errorf("expected docChunker to be called, got %d calls", docChunker.CallCount)
		}
		if result.ChunksCount != 1 {
			t.Errorf("expected 1 chunk, got %d", result.ChunksCount)
		}
	})

	t.Run("embedding failure", func(t *testing.T) {
		docEmbed := NewMockDocEmbedder()
		docEmbed.FailOnCall = 1

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: NewMockCodeEmbedder(),
			DocEmbedder:  docEmbed,
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "# Doc",
			Type:     "doc",
			FilePath: "doc.md",
		})

		if err == nil {
			t.Fatal("expected error on embedding failure")
		}
	})
}

// TestIndexer_IndexManual tests manual item indexing
func TestIndexer_IndexManual(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		itemType    string
		expectCode  bool // true = uses CodeEmbedder, false = uses DocEmbedder
	}{
		{"pattern uses code embedder", "pattern", true},
		{"failure uses code embedder", "failure", true},
		{"decision uses doc embedder", "decision", false},
		{"context uses doc embedder", "context", false},
		{"empty type defaults to context", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codeEmbed := NewMockCodeEmbedder()
			docEmbed := NewMockDocEmbedder()

			idx, _ := NewIndexerWithConfig(IndexerConfig{
				CodeEmbedder: codeEmbed,
				DocEmbedder:  docEmbed,
				VectorStore:  NewMockVectorStorage(),
				MetaStore:    NewMockMetadataStorage(),
				CodeChunker:  NewMockCodeChunker(),
			})

			_, err := idx.IndexFile(ctx, IndexRequest{
				Content: "Some manual content",
				Type:    tt.itemType,
			})

			if err != nil {
				t.Fatalf("IndexFile failed: %v", err)
			}

			if tt.expectCode {
				if codeEmbed.CallCount != 1 {
					t.Errorf("expected CodeEmbedder to be called")
				}
				if docEmbed.CallCount != 0 {
					t.Errorf("expected DocEmbedder NOT to be called")
				}
			} else {
				if docEmbed.CallCount != 1 {
					t.Errorf("expected DocEmbedder to be called")
				}
				if codeEmbed.CallCount != 0 {
					t.Errorf("expected CodeEmbedder NOT to be called")
				}
			}
		})
	}
}

// TestIndexer_TypeRouting tests that IndexFile routes to correct handler
func TestIndexer_TypeRouting(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		req      IndexRequest
		wantType string
	}{
		{
			name:     "explicit code type",
			req:      IndexRequest{Content: "x", Type: "code", FilePath: "any.txt"},
			wantType: "code",
		},
		{
			name:     "explicit doc type",
			req:      IndexRequest{Content: "x", Type: "doc", FilePath: "any.go"},
			wantType: "doc",
		},
		{
			name:     "detected code from .go",
			req:      IndexRequest{Content: "x", FilePath: "main.go"},
			wantType: "code",
		},
		{
			name:     "detected doc from .md",
			req:      IndexRequest{Content: "x", FilePath: "README.md"},
			wantType: "doc",
		},
		{
			name:     "manual for unknown extension",
			req:      IndexRequest{Content: "x", FilePath: "config.yaml"},
			wantType: "manual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codeChunker := NewMockCodeChunker()
			docEmbed := NewMockDocEmbedder()
			codeEmbed := NewMockCodeEmbedder()

			idx, _ := NewIndexerWithConfig(IndexerConfig{
				CodeEmbedder: codeEmbed,
				DocEmbedder:  docEmbed,
				VectorStore:  NewMockVectorStorage(),
				MetaStore:    NewMockMetadataStorage(),
				CodeChunker:  codeChunker,
			})

			_, _ = idx.IndexFile(ctx, tt.req)

			switch tt.wantType {
			case "code":
				if codeChunker.CallCount == 0 {
					t.Error("expected code chunker to be called for code type")
				}
			case "doc":
				if docEmbed.CallCount == 0 {
					t.Error("expected doc embedder to be called for doc type")
				}
			case "manual":
				// Manual uses either embedder based on item type
				if codeEmbed.CallCount == 0 && docEmbed.CallCount == 0 {
					t.Error("expected an embedder to be called for manual type")
				}
			}
		})
	}
}

// TestIndexer_Close tests resource cleanup
func TestIndexer_Close(t *testing.T) {
	codeChunker := NewMockCodeChunker()

	idx, _ := NewIndexerWithConfig(IndexerConfig{
		CodeEmbedder: NewMockCodeEmbedder(),
		DocEmbedder:  NewMockDocEmbedder(),
		VectorStore:  NewMockVectorStorage(),
		MetaStore:    NewMockMetadataStorage(),
		CodeChunker:  codeChunker,
	})

	err := idx.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !codeChunker.Closed {
		t.Error("expected code chunker to be closed")
	}
}

// TestIndexer_MultipleChunks tests indexing content that produces multiple chunks
func TestIndexer_MultipleChunks(t *testing.T) {
	ctx := context.Background()

	// Custom chunker that returns multiple chunks
	codeChunker := NewMockCodeChunker()
	codeChunker.ChunkFunc = func(content []byte, lang, filePath string) ([]chunking.CodeChunk, error) {
		return []chunking.CodeChunk{
			{Content: "chunk1", Type: "function", Name: "func1", StartLine: 1, EndLine: 5},
			{Content: "chunk2", Type: "function", Name: "func2", StartLine: 6, EndLine: 10},
			{Content: "chunk3", Type: "function", Name: "func3", StartLine: 11, EndLine: 15},
		}, nil
	}

	vectorStore := NewMockVectorStorage()
	metaStore := NewMockMetadataStorage()

	idx, _ := NewIndexerWithConfig(IndexerConfig{
		CodeEmbedder: NewMockCodeEmbedder(),
		DocEmbedder:  NewMockDocEmbedder(),
		VectorStore:  vectorStore,
		MetaStore:    metaStore,
		CodeChunker:  codeChunker,
	})

	result, err := idx.IndexFile(ctx, IndexRequest{
		Content:  "func1\nfunc2\nfunc3",
		Type:     "code",
		FilePath: "multi.go",
	})

	if err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}
	if result.ChunksCount != 3 {
		t.Errorf("expected 3 chunks, got %d", result.ChunksCount)
	}
	if vectorStore.UpsertCount != 3 {
		t.Errorf("expected 3 upserts, got %d", vectorStore.UpsertCount)
	}
	if metaStore.SaveCount != 3 {
		t.Errorf("expected 3 saves, got %d", metaStore.SaveCount)
	}
}

// TestIndexer_ErrorWrapping verifies errors are properly wrapped with context
func TestIndexer_ErrorWrapping(t *testing.T) {
	ctx := context.Background()

	t.Run("embedding error includes chunk number", func(t *testing.T) {
		codeEmbed := NewMockCodeEmbedder()
		codeEmbed.FailOnCall = 1

		idx, _ := NewIndexerWithConfig(IndexerConfig{
			CodeEmbedder: codeEmbed,
			DocEmbedder:  NewMockDocEmbedder(),
			VectorStore:  NewMockVectorStorage(),
			MetaStore:    NewMockMetadataStorage(),
			CodeChunker:  NewMockCodeChunker(),
		})

		_, err := idx.IndexFile(ctx, IndexRequest{
			Content:  "code",
			Type:     "code",
			FilePath: "test.go",
		})

		if err == nil {
			t.Fatal("expected error")
		}
		// Error should be wrapped
		if !errors.Is(err, ErrMockEmbedding) {
			t.Errorf("error should wrap ErrMockEmbedding: %v", err)
		}
		// Error message should include context
		if !contains(err.Error(), "chunk") || !contains(err.Error(), "embed") {
			t.Errorf("error should mention chunk and embed: %v", err)
		}
	})
}

// Edge case tests for detectContentType
func TestDetectContentType_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  string
		want     string
	}{
		// Case sensitivity
		{"uppercase GO", "main.GO", "", "code"},
		{"uppercase MD", "README.MD", "", "doc"},
		{"mixed case Py", "Script.Py", "", "code"},

		// Paths with directories
		{"nested Go file", "/src/internal/pkg/handler.go", "", "code"},
		{"nested markdown", "docs/api/README.md", "", "doc"},

		// Edge cases
		{"empty path", "", "", "manual"},
		{"dot only", ".", "", "manual"},
		{"hidden file with ext", ".hidden.go", "", "code"},

		// Multiple extensions (uses last)
		{"tar.gz", "archive.tar.gz", "", "manual"},
		{"test.spec.ts", "component.spec.ts", "", "code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.filePath, tt.content)
			if got != tt.want {
				t.Errorf("detectContentType(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

// Edge case tests for isIndexable
func TestIsIndexable_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Hidden files
		{"hidden go file", ".hidden.go", true},
		{"hidden md file", ".config.md", true},

		// Nested paths
		{"deeply nested go", "/a/b/c/d/e/f/main.go", true},
		{"node_modules ts", "node_modules/pkg/index.ts", true},

		// Various doc formats
		{"rst file", "docs/index.rst", true},
		{"txt file", "notes.txt", true},

		// Non-indexable
		{"svg file", "icon.svg", false},
		{"pdf file", "manual.pdf", false},
		{"lock file", "package-lock.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIndexable(tt.path)
			if got != tt.want {
				t.Errorf("isIndexable(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// Edge case tests for extractTitle
func TestExtractTitle_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		// Unicode content
		{
			name:    "unicode title",
			content: "Título en español\nContenido aquí",
			want:    "Título en español",
		},
		{
			name:    "emoji in title",
			content: "🚀 Launch Guide\nHow to deploy",
			want:    "🚀 Launch Guide",
		},
		{
			name:    "CJK characters",
			content: "日本語タイトル\n内容",
			want:    "日本語タイトル",
		},

		// Special formatting
		{
			name:    "markdown header",
			content: "# My Header\nBody text",
			want:    "# My Header",
		},
		{
			name:    "code block first line",
			content: "```go\nfunc main() {}\n```",
			want:    "```go",
		},

		// Whitespace handling
		{
			name:    "tabs and spaces",
			content: "\t  \n  \t\nActual content",
			want:    "Actual content",
		},
		{
			name:    "CRLF line endings",
			content: "Title\r\nBody",
			want:    "Title",
		},

		// Boundary: exactly 100 characters
		{
			name:    "exactly 100 chars",
			content: "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			want:    "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name:    "101 chars truncates",
			content: "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901",
			want:    "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Tests for basicDocChunking edge cases
func TestBasicDocChunking_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantMinChunks int
		wantMaxChunks int
	}{
		{
			name:         "empty content",
			content:      "",
			wantMinChunks: 0,
			wantMaxChunks: 0,
		},
		{
			name:         "whitespace only",
			content:      "   \n\n\t\n   ",
			wantMinChunks: 0,
			wantMaxChunks: 0,
		},
		{
			name:         "single line no header",
			content:      "Just a single line of text",
			wantMinChunks: 1,
			wantMaxChunks: 1,
		},
		{
			name:         "single header only",
			content:      "# Header Only",
			wantMinChunks: 0, // No content under header
			wantMaxChunks: 1,
		},
		{
			name: "deeply nested headers",
			content: `# H1
## H2
### H3
#### H4
##### H5
###### H6
Content at level 6`,
			wantMinChunks: 1,
			wantMaxChunks: 6,
		},
		{
			name: "code blocks preserved",
			content: `# Code Example

` + "```go" + `
func main() {
    fmt.Println("Hello")
}
` + "```" + `

More text here.`,
			wantMinChunks: 1,
			wantMaxChunks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := basicDocChunking(tt.content, "/test.md")

			if len(chunks) < tt.wantMinChunks {
				t.Errorf("got %d chunks, want at least %d", len(chunks), tt.wantMinChunks)
			}
			if len(chunks) > tt.wantMaxChunks {
				t.Errorf("got %d chunks, want at most %d", len(chunks), tt.wantMaxChunks)
			}
		})
	}
}

// Test buildCodeTitle with various chunk types
func TestBuildCodeTitle_AllTypes(t *testing.T) {
	chunkTypes := []string{"function", "method", "class", "type", "interface", "module", "chunk"}

	for _, chunkType := range chunkTypes {
		t.Run(chunkType+"_with_name", func(t *testing.T) {
			chunk := chunking.CodeChunk{
				Type:     chunkType,
				Name:     "TestName",
				FilePath: "/path/to/file.go",
			}
			got := buildCodeTitle(chunk)
			if got == "" {
				t.Error("expected non-empty title")
			}
			if !contains(got, chunkType) {
				t.Errorf("title %q should contain type %q", got, chunkType)
			}
			if !contains(got, "TestName") {
				t.Errorf("title %q should contain name TestName", got)
			}
		})

		t.Run(chunkType+"_without_name", func(t *testing.T) {
			chunk := chunking.CodeChunk{
				Type:      chunkType,
				Name:      "",
				FilePath:  "/path/to/file.go",
				StartLine: 1,
				EndLine:   10,
			}
			got := buildCodeTitle(chunk)
			if got == "" {
				t.Error("expected non-empty title")
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests
func BenchmarkDetectContentType(b *testing.B) {
	paths := []string{
		"main.go",
		"README.md",
		"config.json",
		"src/components/Button.tsx",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			detectContentType(path, "")
		}
	}
}

func BenchmarkExtractTitle(b *testing.B) {
	content := `# My Document Title

This is the first paragraph of the document.
It contains multiple lines of text.

## Section One

More content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractTitle(content)
	}
}

// Test helper function for filepath operations
func TestFilepathBase(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.go", "file.go"},
		{"file.go", "file.go"},
		{"/a/b/c/d/e.txt", "e.txt"},
	}

	for _, tt := range tests {
		got := filepath.Base(tt.path)
		if got != tt.want {
			t.Errorf("filepath.Base(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
