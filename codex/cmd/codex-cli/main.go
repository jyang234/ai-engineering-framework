// codex-cli is the admin CLI for Codex v1 knowledge management
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	verbose bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "codex-cli",
	Short: "Codex v1 knowledge management CLI",
	Long: `Codex CLI provides administration tools for the Codex v1 knowledge base.

Commands:
  index    - Index files or directories into Codex
  search   - Search the knowledge base
  migrate  - Migrate from RECALL v0 (SQLite FTS) to Codex v1 (Qdrant)
  status   - Show system status and statistics
  serve    - Start MCP server and/or web UI

Environment Variables:
  QDRANT_ADDR        Qdrant server address (default: localhost:6334)
  CODEX_COLLECTION   Collection name (default: codex_v1)
  VOYAGE_API_KEY     Voyage AI API key (required for code indexing)
  OPENAI_API_KEY     OpenAI API key (required for doc indexing)
  ANTHROPIC_API_KEY  Anthropic API key (optional, contextual enrichment)
  CODEX_MODELS_PATH  Path to reranking models (optional)
  CODEX_METADATA_DB  SQLite metadata DB path (default: ~/.codex/metadata.db)`,
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serveCmd)
}
