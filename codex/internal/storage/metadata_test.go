package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestMetadataStore creates an in-memory SQLite database for testing
func createTestMetadataStore(t *testing.T) (*MetadataStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create MetadataStore: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

// seedTestItems inserts test items into the store for testing
func seedTestItems(t *testing.T, store *MetadataStore, items []*ItemRecord) {
	t.Helper()
	for _, item := range items {
		if err := store.SaveItem(item); err != nil {
			t.Fatalf("Failed to seed item %s: %v", item.ID, err)
		}
	}
}

// makeTestItem creates an ItemRecord with sensible defaults
func makeTestItem(id, itemType, scope string) *ItemRecord {
	now := time.Now()
	return &ItemRecord{
		ID:        id,
		Type:      itemType,
		Title:     "Test " + id,
		Content:   "Content for " + id,
		Tags:      []string{"test"},
		Scope:     scope,
		Source:    "test",
		Metadata:  map[string]any{"test": true},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// =============================================================================
// TestListItems - filtering by type, scope, pagination
// =============================================================================

func TestListItems(t *testing.T) {
	tests := []struct {
		name      string
		setup     []*ItemRecord
		itemType  string
		scope     string
		limit     int
		offset    int
		wantCount int
		wantIDs   []string // Expected IDs in order (most recent first)
	}{
		// Happy path tests
		{
			name: "Given items exist When listing all items Then returns all items",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "failure", "project"),
				makeTestItem("3", "decision", "global"),
			},
			itemType:  "",
			scope:     "",
			limit:     50,
			offset:    0,
			wantCount: 3,
		},
		{
			name: "Given items exist When filtering by type Then returns only matching type",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "failure", "project"),
				makeTestItem("3", "pattern", "project"),
			},
			itemType:  "pattern",
			scope:     "",
			limit:     50,
			offset:    0,
			wantCount: 2,
		},
		{
			name: "Given items exist When filtering by scope Then returns only matching scope",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "global"),
				makeTestItem("3", "pattern", "project"),
			},
			itemType:  "",
			scope:     "global",
			limit:     50,
			offset:    0,
			wantCount: 1,
		},
		{
			name: "Given items exist When filtering by type and scope Then returns matching items",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "global"),
				makeTestItem("3", "failure", "global"),
			},
			itemType:  "pattern",
			scope:     "global",
			limit:     50,
			offset:    0,
			wantCount: 1,
		},

		// Pagination tests
		{
			name: "Given many items When using limit Then returns limited items",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
				makeTestItem("3", "pattern", "project"),
				makeTestItem("4", "pattern", "project"),
				makeTestItem("5", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     3,
			offset:    0,
			wantCount: 3,
		},
		{
			name: "Given many items When using offset Then skips items",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
				makeTestItem("3", "pattern", "project"),
				makeTestItem("4", "pattern", "project"),
				makeTestItem("5", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     50,
			offset:    2,
			wantCount: 3,
		},
		{
			name: "Given many items When using limit and offset Then paginates correctly",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
				makeTestItem("3", "pattern", "project"),
				makeTestItem("4", "pattern", "project"),
				makeTestItem("5", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     2,
			offset:    2,
			wantCount: 2,
		},

		// Negative/edge case tests
		{
			name:      "Given empty database When listing items Then returns empty slice",
			setup:     []*ItemRecord{},
			itemType:  "",
			scope:     "",
			limit:     50,
			offset:    0,
			wantCount: 0,
		},
		{
			name: "Given items exist When filtering by nonexistent type Then returns empty",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
			},
			itemType:  "nonexistent",
			scope:     "",
			limit:     50,
			offset:    0,
			wantCount: 0,
		},
		{
			name: "Given items exist When offset exceeds count Then returns empty",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     50,
			offset:    100,
			wantCount: 0,
		},
		{
			name: "Given items exist When limit is zero Then uses default limit",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     0,
			offset:    0,
			wantCount: 1, // Default limit of 50 should still return the item
		},
		{
			name: "Given items exist When limit is negative Then uses default limit",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
			},
			itemType:  "",
			scope:     "",
			limit:     -1,
			offset:    0,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			store, cleanup := createTestMetadataStore(t)
			defer cleanup()

			if len(tt.setup) > 0 {
				seedTestItems(t, store, tt.setup)
			}

			// When
			items, err := store.ListItems(tt.itemType, tt.scope, tt.limit, tt.offset)

			// Then
			if err != nil {
				t.Fatalf("ListItems() error = %v", err)
			}

			if len(items) != tt.wantCount {
				t.Errorf("ListItems() returned %d items, want %d", len(items), tt.wantCount)
			}
		})
	}
}

func TestListItems_OrderByUpdatedAt(t *testing.T) {
	// Given - items with different update times
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	baseTime := time.Now()
	items := []*ItemRecord{
		{
			ID: "old", Type: "pattern", Title: "Old", Content: "c",
			Scope: "project", CreatedAt: baseTime, UpdatedAt: baseTime.Add(-2 * time.Hour),
		},
		{
			ID: "newest", Type: "pattern", Title: "Newest", Content: "c",
			Scope: "project", CreatedAt: baseTime, UpdatedAt: baseTime,
		},
		{
			ID: "middle", Type: "pattern", Title: "Middle", Content: "c",
			Scope: "project", CreatedAt: baseTime, UpdatedAt: baseTime.Add(-1 * time.Hour),
		},
	}
	seedTestItems(t, store, items)

	// When
	result, err := store.ListItems("", "", 50, 0)

	// Then
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(result))
	}

	// Should be ordered by updated_at DESC (newest first)
	expectedOrder := []string{"newest", "middle", "old"}
	for i, id := range expectedOrder {
		if result[i].ID != id {
			t.Errorf("Item at position %d: expected ID %q, got %q", i, id, result[i].ID)
		}
	}
}

func TestListItems_PreservesAllFields(t *testing.T) {
	// Given - an item with all fields populated
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second)
	item := &ItemRecord{
		ID:        "full-item",
		Type:      "pattern",
		Title:     "Full Item Title",
		Content:   "Full content here",
		Tags:      []string{"tag1", "tag2", "tag3"},
		Scope:     "global",
		Source:    "test-source",
		Metadata:  map[string]any{"key1": "value1", "key2": float64(42)},
		CreatedAt: now,
		UpdatedAt: now,
	}
	seedTestItems(t, store, []*ItemRecord{item})

	// When
	items, err := store.ListItems("", "", 50, 0)

	// Then
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	got := items[0]
	if got.ID != item.ID {
		t.Errorf("ID: got %q, want %q", got.ID, item.ID)
	}
	if got.Type != item.Type {
		t.Errorf("Type: got %q, want %q", got.Type, item.Type)
	}
	if got.Title != item.Title {
		t.Errorf("Title: got %q, want %q", got.Title, item.Title)
	}
	if got.Content != item.Content {
		t.Errorf("Content: got %q, want %q", got.Content, item.Content)
	}
	if got.Scope != item.Scope {
		t.Errorf("Scope: got %q, want %q", got.Scope, item.Scope)
	}
	if got.Source != item.Source {
		t.Errorf("Source: got %q, want %q", got.Source, item.Source)
	}
	if len(got.Tags) != len(item.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(got.Tags), len(item.Tags))
	}
	if got.Metadata["key1"] != item.Metadata["key1"] {
		t.Errorf("Metadata key1: got %v, want %v", got.Metadata["key1"], item.Metadata["key1"])
	}
}

// =============================================================================
// TestDeleteItem - deletion, missing item
// =============================================================================

func TestDeleteItem(t *testing.T) {
	tests := []struct {
		name       string
		setup      []*ItemRecord
		deleteID   string
		wantErr    bool
		wantCount  int // Expected count after deletion
	}{
		// Happy path
		{
			name: "Given item exists When deleting Then item is removed",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "failure", "project"),
			},
			deleteID:  "1",
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Given single item When deleting Then database is empty",
			setup: []*ItemRecord{
				makeTestItem("only-one", "pattern", "project"),
			},
			deleteID:  "only-one",
			wantErr:   false,
			wantCount: 0,
		},

		// Negative path - deleting nonexistent item
		{
			name: "Given item does not exist When deleting Then no error",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
			},
			deleteID:  "nonexistent",
			wantErr:   false, // SQLite DELETE doesn't error on missing rows
			wantCount: 1,     // Original item should still be there
		},

		// Edge case - empty database
		{
			name:      "Given empty database When deleting Then no error",
			setup:     []*ItemRecord{},
			deleteID:  "any",
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			store, cleanup := createTestMetadataStore(t)
			defer cleanup()

			if len(tt.setup) > 0 {
				seedTestItems(t, store, tt.setup)
			}

			// When
			err := store.DeleteItem(tt.deleteID)

			// Then
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteItem() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify count
			count, _ := store.CountItems("")
			if count != tt.wantCount {
				t.Errorf("After deletion: got %d items, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestDeleteItem_VerifyDeletion(t *testing.T) {
	// Given
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	item := makeTestItem("to-delete", "pattern", "project")
	seedTestItems(t, store, []*ItemRecord{item})

	// Verify item exists
	_, err := store.GetItem("to-delete")
	if err != nil {
		t.Fatalf("Item should exist before deletion: %v", err)
	}

	// When
	err = store.DeleteItem("to-delete")
	if err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}

	// Then - item should no longer be retrievable
	_, err = store.GetItem("to-delete")
	if err == nil {
		t.Error("GetItem() should return error for deleted item")
	}
}

func TestDeleteItem_SpecialCharacters(t *testing.T) {
	// Given - IDs with special characters
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	specialIDs := []string{
		"item-with-dashes",
		"item_with_underscores",
		"item.with.dots",
		"item/with/slashes",
		"item with spaces",
		"unicode-\u4e2d\u6587",
	}

	for _, id := range specialIDs {
		item := makeTestItem(id, "pattern", "project")
		if err := store.SaveItem(item); err != nil {
			t.Fatalf("Failed to save item %q: %v", id, err)
		}
	}

	// When/Then - each item should be deletable
	for _, id := range specialIDs {
		err := store.DeleteItem(id)
		if err != nil {
			t.Errorf("DeleteItem(%q) error = %v", id, err)
		}

		// Verify deletion
		_, err = store.GetItem(id)
		if err == nil {
			t.Errorf("Item %q should have been deleted", id)
		}
	}
}

// =============================================================================
// TestCountItems - counting with/without filter
// =============================================================================

func TestCountItems(t *testing.T) {
	tests := []struct {
		name      string
		setup     []*ItemRecord
		itemType  string
		wantCount int
	}{
		// Happy path
		{
			name: "Given items exist When counting all Then returns total",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "failure", "project"),
				makeTestItem("3", "decision", "project"),
			},
			itemType:  "",
			wantCount: 3,
		},
		{
			name: "Given items exist When counting by type Then returns filtered count",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
				makeTestItem("3", "failure", "project"),
			},
			itemType:  "pattern",
			wantCount: 2,
		},
		{
			name: "Given single type When counting that type Then returns correct count",
			setup: []*ItemRecord{
				makeTestItem("1", "failure", "project"),
			},
			itemType:  "failure",
			wantCount: 1,
		},

		// Negative path
		{
			name: "Given items exist When counting nonexistent type Then returns zero",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
			},
			itemType:  "nonexistent",
			wantCount: 0,
		},

		// Edge cases
		{
			name:      "Given empty database When counting all Then returns zero",
			setup:     []*ItemRecord{},
			itemType:  "",
			wantCount: 0,
		},
		{
			name:      "Given empty database When counting by type Then returns zero",
			setup:     []*ItemRecord{},
			itemType:  "pattern",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			store, cleanup := createTestMetadataStore(t)
			defer cleanup()

			if len(tt.setup) > 0 {
				seedTestItems(t, store, tt.setup)
			}

			// When
			count, err := store.CountItems(tt.itemType)

			// Then
			if err != nil {
				t.Fatalf("CountItems() error = %v", err)
			}

			if count != tt.wantCount {
				t.Errorf("CountItems(%q) = %d, want %d", tt.itemType, count, tt.wantCount)
			}
		})
	}
}

func TestCountItems_AllTypes(t *testing.T) {
	// Given - items of all known types
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	types := []string{"pattern", "failure", "decision", "context", "runbook", "code", "doc"}
	for i, typ := range types {
		// Create 2 items of each type
		seedTestItems(t, store, []*ItemRecord{
			makeTestItem("a-"+typ, typ, "project"),
			makeTestItem("b-"+typ, typ, "project"),
		})

		// When - count that specific type
		count, err := store.CountItems(typ)

		// Then
		if err != nil {
			t.Fatalf("CountItems(%q) error = %v", typ, err)
		}

		if count != 2 {
			t.Errorf("CountItems(%q) = %d, want 2", typ, count)
		}

		// Verify total count
		total, _ := store.CountItems("")
		expectedTotal := (i + 1) * 2
		if total != expectedTotal {
			t.Errorf("Total count = %d, want %d", total, expectedTotal)
		}
	}
}

func TestCountItems_AfterModifications(t *testing.T) {
	// Given
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	// Initial state
	count, _ := store.CountItems("")
	if count != 0 {
		t.Fatalf("Expected empty database, got %d items", count)
	}

	// When - add items
	seedTestItems(t, store, []*ItemRecord{
		makeTestItem("1", "pattern", "project"),
		makeTestItem("2", "pattern", "project"),
	})

	// Then - count should reflect additions
	count, _ = store.CountItems("")
	if count != 2 {
		t.Errorf("After adds: expected 2 items, got %d", count)
	}

	// When - delete an item
	store.DeleteItem("1")

	// Then - count should reflect deletion
	count, _ = store.CountItems("")
	if count != 1 {
		t.Errorf("After delete: expected 1 item, got %d", count)
	}

	// When - update an item (save with same ID)
	updated := makeTestItem("2", "failure", "global") // Changed type and scope
	store.SaveItem(updated)

	// Then - total count unchanged, but type count changed
	count, _ = store.CountItems("")
	if count != 1 {
		t.Errorf("After update: expected 1 item, got %d", count)
	}

	patternCount, _ := store.CountItems("pattern")
	failureCount, _ := store.CountItems("failure")
	if patternCount != 0 || failureCount != 1 {
		t.Errorf("After update: pattern=%d, failure=%d; want pattern=0, failure=1",
			patternCount, failureCount)
	}
}

// =============================================================================
// TestCountItemsByType - grouped counts
// =============================================================================

func TestCountItemsByType(t *testing.T) {
	tests := []struct {
		name       string
		setup      []*ItemRecord
		wantCounts map[string]int
	}{
		// Happy path
		{
			name: "Given items of different types When counting Then returns grouped counts",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
				makeTestItem("3", "failure", "project"),
				makeTestItem("4", "decision", "project"),
				makeTestItem("5", "decision", "project"),
				makeTestItem("6", "decision", "project"),
			},
			wantCounts: map[string]int{
				"pattern":  2,
				"failure":  1,
				"decision": 3,
			},
		},
		{
			name: "Given single type When counting Then returns that type only",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "pattern", "project"),
			},
			wantCounts: map[string]int{
				"pattern": 2,
			},
		},

		// Edge cases
		{
			name:       "Given empty database When counting Then returns empty map",
			setup:      []*ItemRecord{},
			wantCounts: map[string]int{},
		},
		{
			name: "Given one item per type When counting Then each type has count 1",
			setup: []*ItemRecord{
				makeTestItem("1", "pattern", "project"),
				makeTestItem("2", "failure", "project"),
				makeTestItem("3", "decision", "project"),
				makeTestItem("4", "context", "project"),
			},
			wantCounts: map[string]int{
				"pattern":  1,
				"failure":  1,
				"decision": 1,
				"context":  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			store, cleanup := createTestMetadataStore(t)
			defer cleanup()

			if len(tt.setup) > 0 {
				seedTestItems(t, store, tt.setup)
			}

			// When
			counts, err := store.CountItemsByType()

			// Then
			if err != nil {
				t.Fatalf("CountItemsByType() error = %v", err)
			}

			if len(counts) != len(tt.wantCounts) {
				t.Errorf("CountItemsByType() returned %d types, want %d",
					len(counts), len(tt.wantCounts))
			}

			for typ, wantCount := range tt.wantCounts {
				if gotCount, ok := counts[typ]; !ok {
					t.Errorf("CountItemsByType() missing type %q", typ)
				} else if gotCount != wantCount {
					t.Errorf("CountItemsByType()[%q] = %d, want %d", typ, gotCount, wantCount)
				}
			}
		})
	}
}

func TestCountItemsByType_ConsistentWithCountItems(t *testing.T) {
	// Given - items of various types
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	seedTestItems(t, store, []*ItemRecord{
		makeTestItem("1", "pattern", "project"),
		makeTestItem("2", "pattern", "project"),
		makeTestItem("3", "pattern", "project"),
		makeTestItem("4", "failure", "project"),
		makeTestItem("5", "failure", "project"),
		makeTestItem("6", "decision", "project"),
	})

	// When
	typeCounts, err := store.CountItemsByType()
	if err != nil {
		t.Fatalf("CountItemsByType() error = %v", err)
	}

	totalCount, err := store.CountItems("")
	if err != nil {
		t.Fatalf("CountItems(\"\") error = %v", err)
	}

	// Then - sum of type counts should equal total count
	sum := 0
	for typ, count := range typeCounts {
		sum += count

		// Verify individual type count matches CountItems
		individualCount, err := store.CountItems(typ)
		if err != nil {
			t.Fatalf("CountItems(%q) error = %v", typ, err)
		}
		if individualCount != count {
			t.Errorf("Mismatch for type %q: CountItemsByType=%d, CountItems=%d",
				typ, count, individualCount)
		}
	}

	if sum != totalCount {
		t.Errorf("Sum of type counts (%d) != total count (%d)", sum, totalCount)
	}
}

func TestCountItemsByType_AfterModifications(t *testing.T) {
	// Given
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	// Initial state
	counts, _ := store.CountItemsByType()
	if len(counts) != 0 {
		t.Errorf("Expected empty counts, got %v", counts)
	}

	// When - add items of different types
	seedTestItems(t, store, []*ItemRecord{
		makeTestItem("1", "pattern", "project"),
		makeTestItem("2", "failure", "project"),
	})

	// Then
	counts, _ = store.CountItemsByType()
	if counts["pattern"] != 1 || counts["failure"] != 1 {
		t.Errorf("After adds: expected pattern=1, failure=1; got %v", counts)
	}

	// When - delete the pattern item
	store.DeleteItem("1")

	// Then
	counts, _ = store.CountItemsByType()
	if _, ok := counts["pattern"]; ok {
		t.Error("After delete: pattern type should not exist in counts")
	}
	if counts["failure"] != 1 {
		t.Errorf("After delete: expected failure=1, got %v", counts)
	}

	// When - change item type via update
	updated := makeTestItem("2", "decision", "project")
	store.SaveItem(updated)

	// Then
	counts, _ = store.CountItemsByType()
	if _, ok := counts["failure"]; ok {
		t.Error("After update: failure type should not exist")
	}
	if counts["decision"] != 1 {
		t.Errorf("After update: expected decision=1, got %v", counts)
	}
}

// =============================================================================
// Integration tests - multiple operations
// =============================================================================

func TestMetadataStore_IntegrationScenario(t *testing.T) {
	// This test simulates a realistic usage scenario
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	// Phase 1: Populate the store
	patterns := []*ItemRecord{
		makeTestItem("p1", "pattern", "global"),
		makeTestItem("p2", "pattern", "global"),
		makeTestItem("p3", "pattern", "project"),
	}
	failures := []*ItemRecord{
		makeTestItem("f1", "failure", "project"),
		makeTestItem("f2", "failure", "project"),
	}

	seedTestItems(t, store, patterns)
	seedTestItems(t, store, failures)

	// Verify initial state
	total, _ := store.CountItems("")
	if total != 5 {
		t.Errorf("Initial: expected 5 items, got %d", total)
	}

	// Phase 2: Query with filters
	globalPatterns, _ := store.ListItems("pattern", "global", 50, 0)
	if len(globalPatterns) != 2 {
		t.Errorf("Expected 2 global patterns, got %d", len(globalPatterns))
	}

	projectFailures, _ := store.ListItems("failure", "project", 50, 0)
	if len(projectFailures) != 2 {
		t.Errorf("Expected 2 project failures, got %d", len(projectFailures))
	}

	// Phase 3: Paginate through all items
	page1, _ := store.ListItems("", "", 3, 0)
	page2, _ := store.ListItems("", "", 3, 3)

	if len(page1) != 3 {
		t.Errorf("Page 1: expected 3 items, got %d", len(page1))
	}
	if len(page2) != 2 {
		t.Errorf("Page 2: expected 2 items, got %d", len(page2))
	}

	// Phase 4: Get counts by type
	counts, _ := store.CountItemsByType()
	if counts["pattern"] != 3 || counts["failure"] != 2 {
		t.Errorf("Type counts: expected pattern=3, failure=2; got %v", counts)
	}

	// Phase 5: Delete some items
	store.DeleteItem("p1")
	store.DeleteItem("f1")

	// Verify final state
	total, _ = store.CountItems("")
	if total != 3 {
		t.Errorf("Final: expected 3 items, got %d", total)
	}

	counts, _ = store.CountItemsByType()
	if counts["pattern"] != 2 || counts["failure"] != 1 {
		t.Errorf("Final type counts: expected pattern=2, failure=1; got %v", counts)
	}
}

// =============================================================================
// Benchmark tests
// =============================================================================

func BenchmarkListItems(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench.db")
	store, _ := NewMetadataStore(dbPath)
	defer store.Close()

	// Seed with 1000 items
	for i := 0; i < 1000; i++ {
		item := makeTestItem(
			fmt.Sprintf("item-%d", i),
			[]string{"pattern", "failure", "decision"}[i%3],
			[]string{"project", "global"}[i%2],
		)
		store.SaveItem(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.ListItems("pattern", "", 50, 0)
	}
}

func BenchmarkCountItems(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench.db")
	store, _ := NewMetadataStore(dbPath)
	defer store.Close()

	// Seed with 1000 items
	for i := 0; i < 1000; i++ {
		item := makeTestItem(
			fmt.Sprintf("item-%d", i),
			[]string{"pattern", "failure", "decision"}[i%3],
			"project",
		)
		store.SaveItem(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.CountItems("pattern")
	}
}

func BenchmarkCountItemsByType(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench.db")
	store, _ := NewMetadataStore(dbPath)
	defer store.Close()

	// Seed with 1000 items of various types
	types := []string{"pattern", "failure", "decision", "context", "runbook"}
	for i := 0; i < 1000; i++ {
		item := makeTestItem(
			fmt.Sprintf("item-%d", i),
			types[i%len(types)],
			"project",
		)
		store.SaveItem(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.CountItemsByType()
	}
}

// =============================================================================
// FTS5 Keyword Search tests
// =============================================================================

func TestMetadataStore_KeywordSearch_Basic(t *testing.T) {
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	store.SaveItem(&ItemRecord{
		ID:      "item1",
		Type:    "pattern",
		Title:   "JWT Authentication Pattern",
		Content: "Use JSON Web Tokens for stateless authentication",
		Tags:    []string{"auth", "jwt"},
		Scope:   "global",
	})
	store.SaveItem(&ItemRecord{
		ID:      "item2",
		Type:    "decision",
		Title:   "Database Selection",
		Content: "We chose PostgreSQL for relational data storage",
		Tags:    []string{"database"},
		Scope:   "project",
	})

	results, err := store.KeywordSearch("authentication", 10)
	if err != nil {
		t.Fatalf("KeywordSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "item1" {
		t.Errorf("expected item1, got %s", results[0].ID)
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", results[0].Score)
	}
}

func TestMetadataStore_KeywordSearch_NoResults(t *testing.T) {
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	store.SaveItem(&ItemRecord{
		ID:      "item1",
		Type:    "pattern",
		Title:   "Test Item",
		Content: "Some content",
		Scope:   "global",
	})

	results, err := store.KeywordSearch("nonexistent", 10)
	if err != nil {
		t.Fatalf("KeywordSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMetadataStore_KeywordSearch_SpecialChars(t *testing.T) {
	store, cleanup := createTestMetadataStore(t)
	defer cleanup()

	store.SaveItem(&ItemRecord{
		ID:      "item1",
		Type:    "pattern",
		Title:   "Error Handling",
		Content: "Use AND/OR operators carefully",
		Scope:   "global",
	})

	// These special FTS5 operators should not crash
	results, err := store.KeywordSearch("AND OR NOT *", 10)
	if err != nil {
		t.Fatalf("KeywordSearch with special chars failed: %v", err)
	}
	_ = results // just verify no crash

	results, err = store.KeywordSearch(`query with "quotes"`, 10)
	if err != nil {
		t.Fatalf("KeywordSearch with quotes failed: %v", err)
	}
	_ = results
}
