package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/codex/internal/core"
)

var (
	migrateDryRun bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [v0-db-path]",
	Short: "Migrate from RECALL v0 to Codex v1",
	Long: `Migrate knowledge items from RECALL v0 (SQLite FTS) to Codex v1.

The migration:
1. Reads all items from the v0 SQLite database
2. Re-embeds content using appropriate embedding models
3. Stores vectors in the SQLite vector store
4. Preserves metadata in the new SQLite metadata store

Examples:
  codex-cli migrate ~/.recall/recall.db
  codex-cli migrate /path/to/v0.db --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "show what would be migrated without making changes")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	v0Path := args[0]

	// Validate v0 database
	if err := core.ValidateV0Database(v0Path); err != nil {
		return fmt.Errorf("invalid v0 database: %w", err)
	}

	// Get item count
	count, err := core.GetV0ItemCount(v0Path)
	if err != nil {
		return fmt.Errorf("failed to count items: %w", err)
	}

	fmt.Printf("Found %d items in v0 database\n", count)

	if migrateDryRun {
		fmt.Println("Dry run - no changes made")
		return nil
	}

	// Load config
	cfg := LoadConfig()
	if cfg.VoyageAPIKey == "" || cfg.OpenAIAPIKey == "" {
		return fmt.Errorf("VOYAGE_API_KEY and OPENAI_API_KEY required for migration")
	}

	ctx := context.Background()
	engine, err := core.NewSearchEngine(ctx, cfg.ToEngineConfig())
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer engine.Close()

	// Run migration with progress
	fmt.Println("Starting migration...")

	stats, err := engine.MigrateFromV0WithProgress(ctx, v0Path, func(current, total int, item string) {
		if verbose {
			fmt.Printf("[%d/%d] Migrating: %s\n", current+1, total, item)
		} else if current%10 == 0 {
			fmt.Printf("\rProgress: %d/%d items", current, total)
		}
	})

	fmt.Println() // Clear progress line

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Print summary
	core.PrintMigrationStats(stats)

	return nil
}
