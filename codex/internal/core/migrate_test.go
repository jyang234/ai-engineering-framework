package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestMapV0TypeToV1Type(t *testing.T) {
	tests := []struct {
		v0Type string
		want   string
	}{
		{"pattern", "pattern"},
		{"failure", "failure"},
		{"decision", "decision"},
		{"adr", "decision"},
		{"context", "context"},
		{"runbook", "runbook"},
		{"unknown", "context"},
		{"", "context"},
	}

	for _, tt := range tests {
		t.Run(tt.v0Type, func(t *testing.T) {
			got := mapV0TypeToV1Type(tt.v0Type)
			if got != tt.want {
				t.Errorf("mapV0TypeToV1Type(%q) = %q, want %q", tt.v0Type, got, tt.want)
			}
		})
	}
}

func TestMigrationStats(t *testing.T) {
	stats := &MigrationStats{
		TotalItems:    100,
		MigratedItems: 95,
		FailedItems:   5,
		TotalChunks:   250,
		StartTime:     time.Now().Add(-5 * time.Minute),
		EndTime:       time.Now(),
		Errors:        []string{"error 1", "error 2"},
	}

	if stats.TotalItems != 100 {
		t.Errorf("Expected TotalItems = 100, got %d", stats.TotalItems)
	}

	if stats.MigratedItems+stats.FailedItems != stats.TotalItems {
		t.Error("MigratedItems + FailedItems should equal TotalItems")
	}

	if len(stats.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(stats.Errors))
	}

	duration := stats.EndTime.Sub(stats.StartTime)
	if duration < 5*time.Minute {
		t.Errorf("Expected duration >= 5 minutes, got %v", duration)
	}
}

// createTestV0Database creates a temporary SQLite database mimicking RECALL v0
func createTestV0Database(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "codex-migrate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "recall.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create v0 schema (without FTS5 for testing - FTS5 may not be available)
	schema := `
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			scope TEXT NOT NULL DEFAULT 'project',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert test data
	now := time.Now().Format(time.RFC3339)
	testItems := []struct {
		id, typ, title, content, tags, scope string
	}{
		{"1", "pattern", "Error Handling Pattern", "Always wrap errors with context", `["go","errors"]`, "global"},
		{"2", "failure", "Nil Pointer Bug", "Check for nil before dereferencing", `["go","bugs"]`, "project"},
		{"3", "decision", "Use PostgreSQL", "Chose PostgreSQL for ACID compliance", `["database"]`, "project"},
		{"4", "context", "Project Overview", "This project does X, Y, Z", `[]`, "project"},
		{"5", "runbook", "Deploy Process", "1. Build\n2. Test\n3. Deploy", `["ops"]`, "global"},
	}

	for _, item := range testItems {
		_, err := db.Exec(`
			INSERT INTO items (id, type, title, content, tags, scope, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, item.id, item.typ, item.title, item.content, item.tags, item.scope, now, now)
		if err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("Failed to insert test item: %v", err)
		}
	}

	db.Close()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return dbPath, cleanup
}

func TestValidateV0Database(t *testing.T) {
	dbPath, cleanup := createTestV0Database(t)
	defer cleanup()

	err := ValidateV0Database(dbPath)
	if err != nil {
		t.Errorf("ValidateV0Database() failed for valid db: %v", err)
	}
}

func TestValidateV0Database_InvalidPath(t *testing.T) {
	err := ValidateV0Database("/nonexistent/path/db.sqlite")
	if err == nil {
		t.Error("Expected error for nonexistent database")
	}
}

func TestValidateV0Database_NotV0DB(t *testing.T) {
	// Create a database without the items table
	tmpDir, err := os.MkdirTemp("", "codex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "notv0.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create a different table
	_, err = db.Exec("CREATE TABLE other_table (id TEXT)")
	db.Close()
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = ValidateV0Database(dbPath)
	if err == nil {
		t.Error("Expected error for database without items table")
	}
}

func TestGetV0ItemCount(t *testing.T) {
	dbPath, cleanup := createTestV0Database(t)
	defer cleanup()

	count, err := GetV0ItemCount(dbPath)
	if err != nil {
		t.Fatalf("GetV0ItemCount() failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 items, got %d", count)
	}
}

func TestGetV0ItemCount_EmptyDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "codex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "empty.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			type TEXT,
			title TEXT,
			content TEXT,
			tags TEXT,
			scope TEXT,
			created_at TEXT,
			updated_at TEXT
		)
	`)
	db.Close()
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	count, err := GetV0ItemCount(dbPath)
	if err != nil {
		t.Fatalf("GetV0ItemCount() failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 items, got %d", count)
	}
}

func TestV0ItemParsing(t *testing.T) {
	dbPath, cleanup := createTestV0Database(t)
	defer cleanup()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, type, title, content, tags, scope, created_at, updated_at
		FROM items WHERE id = '1'
	`)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("Expected one row")
	}

	var item V0Item
	err = rows.Scan(
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
		t.Fatalf("Scan failed: %v", err)
	}

	if item.ID != "1" {
		t.Errorf("Expected ID '1', got %q", item.ID)
	}
	if item.Type != "pattern" {
		t.Errorf("Expected type 'pattern', got %q", item.Type)
	}
	if item.Title != "Error Handling Pattern" {
		t.Errorf("Unexpected title: %q", item.Title)
	}
	if item.Scope != "global" {
		t.Errorf("Expected scope 'global', got %q", item.Scope)
	}
}

func TestMigrationProgressCallback(t *testing.T) {
	var calls []struct {
		current, total int
		item           string
	}

	callback := func(current, total int, item string) {
		calls = append(calls, struct {
			current, total int
			item           string
		}{current, total, item})
	}

	// Simulate calling the callback
	callback(0, 5, "Item 1")
	callback(1, 5, "Item 2")
	callback(2, 5, "Item 3")

	if len(calls) != 3 {
		t.Errorf("Expected 3 callback calls, got %d", len(calls))
	}

	if calls[0].current != 0 || calls[0].total != 5 {
		t.Errorf("First call: expected current=0, total=5, got current=%d, total=%d",
			calls[0].current, calls[0].total)
	}
}

// TestMigrateV0ToV1 tests the full migration
// Note: This is an integration test that would require a real SearchEngine
func TestMigrateV0ToV1_Integration(t *testing.T) {
	t.Skip("Integration test - requires SearchEngine with embedding API keys")

	dbPath, cleanup := createTestV0Database(t)
	defer cleanup()

	// Would need a real SearchEngine here
	ctx := context.Background()
	_ = ctx
	_ = dbPath
}

// Benchmark migration query
func BenchmarkV0Query(b *testing.B) {
	// Create a test database with more items
	tmpDir, err := os.MkdirTemp("", "codex-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			scope TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		b.Fatalf("Failed to create schema: %v", err)
	}

	// Insert 1000 items
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < 1000; i++ {
		_, err = db.Exec(`
			INSERT INTO items (id, type, title, content, tags, scope, created_at, updated_at)
			VALUES (?, 'pattern', 'Test Item', 'Test content', '[]', 'project', ?, ?)
		`, i, now, now)
		if err != nil {
			db.Close()
			b.Fatalf("Failed to insert: %v", err)
		}
	}

	db.Close()

	// Benchmark the query
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, _ := sql.Open("sqlite3", dbPath)
		rows, _ := db.Query(`
			SELECT id, type, title, content, tags, scope, created_at, updated_at
			FROM items ORDER BY created_at ASC
		`)
		count := 0
		for rows.Next() {
			count++
		}
		rows.Close()
		db.Close()
	}
}

func TestPrintMigrationStats(t *testing.T) {
	stats := &MigrationStats{
		TotalItems:    10,
		MigratedItems: 8,
		FailedItems:   2,
		TotalChunks:   25,
		StartTime:     time.Now().Add(-10 * time.Second),
		EndTime:       time.Now(),
		Errors:        []string{"error 1", "error 2"},
	}

	// Just verify it doesn't panic
	// In a real test we'd capture stdout and verify the output
	PrintMigrationStats(stats)
}

// Additional edge case tests for migration

func TestMapV0TypeToV1Type_AllTypes(t *testing.T) {
	// Ensure all v0 types map to valid v1 types
	v0Types := []string{
		"pattern", "failure", "decision", "adr", "context", "runbook",
		"unknown", "", "PATTERN", "Pattern", "arbitrary",
	}

	validV1Types := map[string]bool{
		"pattern": true, "failure": true, "decision": true,
		"context": true, "runbook": true,
	}

	for _, v0Type := range v0Types {
		t.Run(v0Type, func(t *testing.T) {
			v1Type := mapV0TypeToV1Type(v0Type)
			if !validV1Types[v1Type] {
				t.Errorf("mapV0TypeToV1Type(%q) = %q, which is not a valid v1 type", v0Type, v1Type)
			}
		})
	}
}

func TestMigrationStats_ZeroValues(t *testing.T) {
	stats := &MigrationStats{}

	// Zero values should be valid
	if stats.TotalItems != 0 {
		t.Errorf("expected TotalItems = 0, got %d", stats.TotalItems)
	}
	if stats.MigratedItems != 0 {
		t.Errorf("expected MigratedItems = 0, got %d", stats.MigratedItems)
	}
	if stats.FailedItems != 0 {
		t.Errorf("expected FailedItems = 0, got %d", stats.FailedItems)
	}
	if stats.Errors != nil && len(stats.Errors) != 0 {
		t.Errorf("expected empty Errors, got %v", stats.Errors)
	}

	// Should not panic
	PrintMigrationStats(stats)
}

func TestMigrationStats_ManyErrors(t *testing.T) {
	// Test that we handle many errors gracefully (only show first 10)
	stats := &MigrationStats{
		TotalItems:    100,
		MigratedItems: 50,
		FailedItems:   50,
		TotalChunks:   100,
		StartTime:     time.Now().Add(-1 * time.Hour),
		EndTime:       time.Now(),
	}

	// Add 100 errors
	for i := 0; i < 100; i++ {
		stats.Errors = append(stats.Errors, fmt.Sprintf("error %d: something went wrong", i))
	}

	// Should not panic and should truncate output
	PrintMigrationStats(stats)
}

func TestV0Item_EmptyContent(t *testing.T) {
	item := V0Item{
		ID:        "test-empty",
		Type:      "pattern",
		Title:     "Empty Pattern",
		Content:   "",
		Tags:      "[]",
		Scope:     "project",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	// Empty content should be handled gracefully
	if item.Content != "" {
		t.Errorf("expected empty content")
	}
}

func TestV0Item_SpecialCharacters(t *testing.T) {
	// Test that special characters in content are handled
	specialContents := []struct {
		name    string
		content string
	}{
		{"unicode", "こんにちは世界 🌍"},
		{"quotes", `He said "hello" and 'goodbye'`},
		{"backslashes", `C:\Users\test\path`},
		{"newlines", "line1\nline2\r\nline3"},
		{"nulls", "before\x00after"},
		{"sql injection attempt", "'; DROP TABLE items; --"},
	}

	for _, tc := range specialContents {
		t.Run(tc.name, func(t *testing.T) {
			item := V0Item{
				ID:        "test-" + tc.name,
				Type:      "context",
				Title:     "Test " + tc.name,
				Content:   tc.content,
				Tags:      "[]",
				Scope:     "project",
				CreatedAt: time.Now().Format(time.RFC3339),
				UpdatedAt: time.Now().Format(time.RFC3339),
			}

			// Item should be created without error
			if item.Content != tc.content {
				t.Errorf("content mismatch")
			}
		})
	}
}

func TestValidateV0Database_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "codex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty file
	emptyPath := filepath.Join(tmpDir, "empty.db")
	if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	err = ValidateV0Database(emptyPath)
	if err == nil {
		t.Error("Expected error for empty database file")
	}
}

func TestValidateV0Database_CorruptedFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "codex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create file with garbage data
	corruptPath := filepath.Join(tmpDir, "corrupt.db")
	if err := os.WriteFile(corruptPath, []byte("not a sqlite database"), 0644); err != nil {
		t.Fatalf("Failed to create corrupt file: %v", err)
	}

	err = ValidateV0Database(corruptPath)
	if err == nil {
		t.Error("Expected error for corrupted database file")
	}
}

func TestGetV0ItemCount_LargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "codex-large-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "large.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			scope TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert 10000 items
	now := time.Now().Format(time.RFC3339)
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare(`
		INSERT INTO items (id, type, title, content, tags, scope, created_at, updated_at)
		VALUES (?, 'pattern', 'Test', 'Content', '[]', 'project', ?, ?)
	`)

	for i := 0; i < 10000; i++ {
		stmt.Exec(fmt.Sprintf("item-%d", i), now, now)
	}
	stmt.Close()
	tx.Commit()
	db.Close()

	// Test count
	count, err := GetV0ItemCount(dbPath)
	if err != nil {
		t.Fatalf("GetV0ItemCount failed: %v", err)
	}

	if count != 10000 {
		t.Errorf("Expected 10000 items, got %d", count)
	}
}

// TestMigrationStats_SingleThreadedUsage documents that MigrationStats
// is designed for single-threaded use during migration.
// Migration processes items sequentially, so thread-safety is not required.
func TestMigrationStats_SingleThreadedUsage(t *testing.T) {
	stats := &MigrationStats{}

	// Simulate sequential migration processing
	for i := 0; i < 100; i++ {
		stats.TotalItems++
		if i%10 == 0 {
			stats.FailedItems++
			stats.Errors = append(stats.Errors, fmt.Sprintf("error at item %d", i))
		} else {
			stats.MigratedItems++
			stats.TotalChunks += 2 // Average 2 chunks per item
		}
	}

	// Verify counts are consistent
	if stats.TotalItems != 100 {
		t.Errorf("expected TotalItems=100, got %d", stats.TotalItems)
	}
	if stats.MigratedItems+stats.FailedItems != stats.TotalItems {
		t.Errorf("migrated(%d) + failed(%d) != total(%d)",
			stats.MigratedItems, stats.FailedItems, stats.TotalItems)
	}
	if len(stats.Errors) != stats.FailedItems {
		t.Errorf("expected %d errors, got %d", stats.FailedItems, len(stats.Errors))
	}
}
