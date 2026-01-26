package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// MigrationStats tracks migration progress
type MigrationStats struct {
	TotalItems     int
	MigratedItems  int
	FailedItems    int
	SkippedItems   int
	TotalChunks    int
	StartTime      time.Time
	EndTime        time.Time
	Errors         []string
}

// V0Item represents an item from RECALL v0 SQLite FTS database
type V0Item struct {
	ID        string
	Type      string
	Title     string
	Content   string
	Tags      string // JSON array stored as string
	Scope     string
	CreatedAt string
	UpdatedAt string
}

// MigrateV0ToV1 migrates items from RECALL v0 (SQLite FTS) to Codex v1 (Qdrant)
func MigrateV0ToV1(ctx context.Context, v0DBPath string, engine *SearchEngine) (*MigrationStats, error) {
	stats := &MigrationStats{
		StartTime: time.Now(),
	}

	// Open V0 database
	v0DB, err := sql.Open("sqlite3", v0DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open v0 database: %w", err)
	}
	defer v0DB.Close()

	// Create indexer for processing
	indexer, err := NewIndexer(engine)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}
	defer indexer.Close()

	// Query all items from v0
	rows, err := v0DB.Query(`
		SELECT id, type, title, content, tags, scope, created_at, updated_at
		FROM items
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query v0 items: %w", err)
	}
	defer rows.Close()

	// Process items in batches
	var batch []V0Item
	const batchSize = 50

	for rows.Next() {
		var item V0Item
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Title,
			&item.Content,
			&item.Tags,
			&item.Scope,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("scan error: %v", err))
			stats.FailedItems++
			continue
		}

		stats.TotalItems++
		batch = append(batch, item)

		if len(batch) >= batchSize {
			migrateBatch(ctx, batch, indexer, stats)
			batch = batch[:0]
		}
	}

	// Process remaining items
	if len(batch) > 0 {
		migrateBatch(ctx, batch, indexer, stats)
	}

	stats.EndTime = time.Now()
	return stats, nil
}

// migrateBatch processes a batch of V0 items
func migrateBatch(ctx context.Context, items []V0Item, indexer *Indexer, stats *MigrationStats) {
	for _, v0Item := range items {
		result, err := migrateItem(ctx, v0Item, indexer)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("item %s: %v", v0Item.ID, err))
			stats.FailedItems++
			continue
		}

		stats.MigratedItems++
		stats.TotalChunks += result.ChunksCount
	}
}

// migrateItem migrates a single V0 item to V1
func migrateItem(ctx context.Context, v0Item V0Item, indexer *Indexer) (*IndexResult, error) {
	// Skip empty content
	if strings.TrimSpace(v0Item.Content) == "" {
		return &IndexResult{ChunksCount: 0}, nil
	}

	// Parse tags
	var tags []string
	if v0Item.Tags != "" {
		json.Unmarshal([]byte(v0Item.Tags), &tags)
	}

	// Determine content type for routing
	contentType := mapV0TypeToV1Type(v0Item.Type)

	// Create index request
	req := IndexRequest{
		Content:  v0Item.Content,
		Type:     contentType,
		Tags:     tags,
		Scope:    v0Item.Scope,
		FilePath: "", // V0 items may not have file paths
	}

	return indexer.IndexFile(ctx, req)
}

// mapV0TypeToV1Type maps RECALL v0 types to Codex v1 types
func mapV0TypeToV1Type(v0Type string) string {
	switch v0Type {
	case "pattern":
		return "pattern" // Code patterns - use Voyage embeddings
	case "failure":
		return "failure" // Failure patterns - use Voyage embeddings
	case "decision", "adr":
		return "decision" // Decisions/ADRs - use OpenAI embeddings
	case "context":
		return "context" // General context - use OpenAI embeddings
	case "runbook":
		return "runbook" // Runbooks - use OpenAI embeddings
	default:
		return "context" // Default to context
	}
}

// MigrationProgressCallback is called during migration to report progress
type MigrationProgressCallback func(current, total int, item string)

// MigrateV0ToV1WithProgress migrates with progress reporting
func MigrateV0ToV1WithProgress(ctx context.Context, v0DBPath string, engine *SearchEngine, progress MigrationProgressCallback) (*MigrationStats, error) {
	stats := &MigrationStats{
		StartTime: time.Now(),
	}

	// Open V0 database
	v0DB, err := sql.Open("sqlite3", v0DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open v0 database: %w", err)
	}
	defer v0DB.Close()

	// Count total items first
	var total int
	err = v0DB.QueryRow("SELECT COUNT(*) FROM items").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count items: %w", err)
	}
	stats.TotalItems = total

	// Create indexer
	indexer, err := NewIndexer(engine)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}
	defer indexer.Close()

	// Query all items
	rows, err := v0DB.Query(`
		SELECT id, type, title, content, tags, scope, created_at, updated_at
		FROM items
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query v0 items: %w", err)
	}
	defer rows.Close()

	current := 0
	for rows.Next() {
		var item V0Item
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Title,
			&item.Content,
			&item.Tags,
			&item.Scope,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("scan error: %v", err))
			stats.FailedItems++
			current++
			continue
		}

		// Report progress
		if progress != nil {
			progress(current, total, item.Title)
		}

		// Migrate item
		result, err := migrateItem(ctx, item, indexer)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("item %s: %v", item.ID, err))
			stats.FailedItems++
		} else {
			stats.MigratedItems++
			stats.TotalChunks += result.ChunksCount
		}

		current++
	}

	stats.EndTime = time.Now()
	return stats, nil
}

// ValidateV0Database checks if the given path is a valid RECALL v0 database
func ValidateV0Database(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("cannot open database: %w", err)
	}
	defer db.Close()

	// Check for items table
	var tableName string
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='items'
	`).Scan(&tableName)
	if err != nil {
		return fmt.Errorf("items table not found - not a valid RECALL v0 database")
	}

	// Check for FTS table (v0 uses FTS5)
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='items_fts'
	`).Scan(&tableName)
	if err != nil {
		// FTS table is optional, just log
		fmt.Println("Note: FTS table not found - database may be a newer format")
	}

	return nil
}

// GetV0ItemCount returns the number of items in a V0 database
func GetV0ItemCount(dbPath string) (int, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	return count, err
}

// PrintMigrationStats prints a summary of migration results
func PrintMigrationStats(stats *MigrationStats) {
	duration := stats.EndTime.Sub(stats.StartTime)

	fmt.Println("\n=== Migration Complete ===")
	fmt.Printf("Duration: %v\n", duration.Round(time.Second))
	fmt.Printf("Total items: %d\n", stats.TotalItems)
	fmt.Printf("Migrated: %d\n", stats.MigratedItems)
	fmt.Printf("Failed: %d\n", stats.FailedItems)
	fmt.Printf("Total chunks created: %d\n", stats.TotalChunks)

	if len(stats.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(stats.Errors))
		for i, err := range stats.Errors {
			if i >= 10 {
				fmt.Printf("  ... and %d more errors\n", len(stats.Errors)-10)
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}
}
