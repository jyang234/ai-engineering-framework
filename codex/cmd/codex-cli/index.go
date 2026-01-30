package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/codex/internal/core"
)

var (
	indexRecursive bool
	indexScope     string
	indexType      string
	indexTags      []string
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index files or directories into Codex",
	Long: `Index files or directories into the Codex knowledge base.

For code files, uses AST-aware chunking and Voyage embeddings.
For documentation files, uses semantic chunking and OpenAI embeddings.

Examples:
  codex-cli index path/to/file.go --scope project --tags "api,auth"
  codex-cli index ./src --recursive --scope project
  codex-cli index README.md --type doc`,
	Args: cobra.ExactArgs(1),
	RunE: runIndex,
}

func init() {
	indexCmd.Flags().BoolVarP(&indexRecursive, "recursive", "r", false, "recursively index directories")
	indexCmd.Flags().StringVarP(&indexScope, "scope", "s", "project", "scope (global or project)")
	indexCmd.Flags().StringVarP(&indexType, "type", "t", "", "content type (code, doc, or auto-detect)")
	indexCmd.Flags().StringSliceVar(&indexTags, "tags", nil, "tags to apply")
}

func runIndex(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	// Load config and create engine
	cfg := LoadConfig()
	// Local Ollama embedders are used — no API keys required for indexing.

	ctx := context.Background()
	engine, err := core.NewSearchEngine(ctx, cfg.ToEngineConfig())
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer engine.Close()

	if info.IsDir() {
		return indexDirectory(ctx, engine, absPath)
	}
	return indexFile(ctx, engine, absPath)
}

func indexFile(ctx context.Context, engine *core.SearchEngine, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	req := core.IndexRequest{
		Content:  string(content),
		FilePath: path,
		Type:     indexType,
		Tags:     indexTags,
		Scope:    indexScope,
	}

	result, err := engine.Index(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to index: %w", err)
	}

	fmt.Printf("Indexed %s: %d chunks created (ID: %s)\n", path, result.ChunksCount, result.ItemID)
	return nil
}

func indexDirectory(ctx context.Context, engine *core.SearchEngine, dirPath string) error {
	if !indexRecursive {
		return fmt.Errorf("directory indexing requires --recursive flag")
	}

	fmt.Printf("Indexing directory: %s\n", dirPath)

	results, err := engine.IndexDirectory(ctx, dirPath, indexScope)
	if err != nil {
		return fmt.Errorf("failed to index directory: %w", err)
	}

	totalChunks := 0
	for _, r := range results {
		totalChunks += r.ChunksCount
	}

	fmt.Printf("Indexed %d files, %d total chunks\n", len(results), totalChunks)
	return nil
}
