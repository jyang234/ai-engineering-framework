package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/codex/internal/core"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status and statistics",
	Long: `Display Codex system status including:
- SQLite storage status
- Item counts by type
- Configuration summary
- API key status`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := LoadConfig()

	fmt.Println("Codex Status")
	fmt.Println(strings.Repeat("=", 40))

	// Configuration
	fmt.Println("\nConfiguration:")
	fmt.Printf("  Metadata:   %s\n", cfg.MetadataDBPath)

	// Embedding Config
	fmt.Println("\nEmbedding:")
	fmt.Printf("  Model:     %s\n", valueOrDefault(cfg.LocalEmbeddingModel, "nomic-embed-text"))
	fmt.Printf("  Anthropic: %s\n", keyStatus(cfg.AnthropicAPIKey))

	// Models
	fmt.Println("\nReranking:")
	if cfg.ModelsPath != "" {
		fmt.Printf("  Models:    %s\n", cfg.ModelsPath)
	} else {
		fmt.Println("  Models:    not configured")
	}

	// Try to connect and get stats
	fmt.Println("\nConnecting to storage...")

	ctx := context.Background()
	engine, err := core.NewSearchEngine(ctx, cfg.ToEngineConfig())
	if err != nil {
		fmt.Printf("  Status:    FAILED (%s)\n", err)
		return nil // Don't fail command, just report status
	}
	defer engine.Close()

	fmt.Println("  Status:    CONNECTED")

	// Get item stats
	stats, err := engine.Stats(ctx)
	if err != nil {
		fmt.Printf("\nItem counts: error (%s)\n", err)
		return nil
	}

	if len(stats) == 0 {
		fmt.Println("\nItems: (empty)")
	} else {
		fmt.Println("\nItems by type:")
		total := 0
		for itemType, count := range stats {
			fmt.Printf("  %-12s %d\n", itemType+":", count)
			total += count
		}
		fmt.Printf("  %-12s %d\n", "TOTAL:", total)
	}

	return nil
}

func valueOrDefault(val, def string) string {
	if val == "" {
		return def + " (default)"
	}
	return val
}

func keyStatus(key string) string {
	if key == "" {
		return "not set"
	}
	// Show first 4 and last 4 chars
	if len(key) > 12 {
		return key[:4] + "..." + key[len(key)-4:] + " (set)"
	}
	return "****** (set)"
}
