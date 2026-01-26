package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthropics/aef/codex/internal/storage"
)

// =============================================================================
// Test: List
// =============================================================================

func TestSearchEngine_List(t *testing.T) {
	ctx := context.Background()

	t.Run("Given items exist When List called with no filters Then returns all items", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:    "item-1",
			Type:  "pattern",
			Title: "Pattern One",
			Scope: "project",
		}
		metaStore.Items["item-2"] = &storage.ItemRecord{
			ID:    "item-2",
			Type:  "failure",
			Title: "Failure One",
			Scope: "global",
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "", "", 10, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("Given items exist When List called with type filter Then returns only matching type", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:    "item-1",
			Type:  "pattern",
			Title: "Pattern One",
			Scope: "project",
		}
		metaStore.Items["item-2"] = &storage.ItemRecord{
			ID:    "item-2",
			Type:  "failure",
			Title: "Failure One",
			Scope: "global",
		}
		metaStore.Items["item-3"] = &storage.ItemRecord{
			ID:    "item-3",
			Type:  "pattern",
			Title: "Pattern Two",
			Scope: "project",
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "pattern", "", 10, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 pattern items, got %d", len(items))
		}
		for _, item := range items {
			if item.Type != "pattern" {
				t.Errorf("expected type 'pattern', got '%s'", item.Type)
			}
		}
	})

	t.Run("Given items exist When List called with scope filter Then returns only matching scope", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:    "item-1",
			Type:  "pattern",
			Scope: "project",
		}
		metaStore.Items["item-2"] = &storage.ItemRecord{
			ID:    "item-2",
			Type:  "failure",
			Scope: "global",
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "", "global", 10, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 global item, got %d", len(items))
		}
		if len(items) > 0 && items[0].Scope != "global" {
			t.Errorf("expected scope 'global', got '%s'", items[0].Scope)
		}
	})

	t.Run("Given items exist When List called with both type and scope filters Then returns matching items", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:    "item-1",
			Type:  "pattern",
			Scope: "project",
		}
		metaStore.Items["item-2"] = &storage.ItemRecord{
			ID:    "item-2",
			Type:  "pattern",
			Scope: "global",
		}
		metaStore.Items["item-3"] = &storage.ItemRecord{
			ID:    "item-3",
			Type:  "failure",
			Scope: "global",
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "pattern", "global", 10, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item matching type=pattern, scope=global, got %d", len(items))
		}
	})

	t.Run("Given items exist When List called with pagination Then returns correct page", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		for i := 0; i < 10; i++ {
			metaStore.Items[string(rune('a'+i))] = &storage.ItemRecord{
				ID:    string(rune('a' + i)),
				Type:  "pattern",
				Title: "Item " + string(rune('A'+i)),
				Scope: "project",
			}
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When - first page
		items, err := engine.List(ctx, "", "", 3, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 3 {
			t.Errorf("expected 3 items on first page, got %d", len(items))
		}

		// When - second page
		items, err = engine.List(ctx, "", "", 3, 3)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 3 {
			t.Errorf("expected 3 items on second page, got %d", len(items))
		}
	})

	t.Run("Given items exist When List called with offset beyond total Then returns empty slice", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{ID: "item-1", Type: "pattern"}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "", "", 10, 100)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items with offset beyond total, got %d", len(items))
		}
	})

	t.Run("Given no items When List called Then returns empty slice", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		items, err := engine.List(ctx, "", "", 10, 0)

		// Then
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("Given metadata store fails When List called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.FailOnList = true

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		_, err := engine.List(ctx, "", "", 10, 0)

		// Then
		if err == nil {
			t.Fatal("expected error when metadata store fails")
		}
	})
}

// =============================================================================
// Test: Update
// =============================================================================

func TestSearchEngine_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("Given item exists When Update called with valid item Then updates successfully", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:        "item-1",
			Type:      "pattern",
			Title:     "Original Title",
			Content:   "Original content",
			Scope:     "project",
			CreatedAt: time.Now().Add(-time.Hour),
			UpdatedAt: time.Now().Add(-time.Hour),
		}

		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Title:   "Updated Title",
			Content: "Updated content",
			Scope:   "project",
		}
		err := engine.Update(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Update failed: %v", err)
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
		// Verify UpdatedAt was set
		if item.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("Given item does not exist When Update called Then returns not found error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		// No items in store

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "nonexistent",
			Type:    "pattern",
			Title:   "Title",
			Content: "Content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error for nonexistent item")
		}
		if !containsStr(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
	})

	t.Run("Given document type item When Update called Then uses doc embedder", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "decision",
		}

		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			openai:   docEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "decision",
			Content: "Decision content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if docEmbed.CallCount != 1 {
			t.Errorf("expected doc embedder to be called, got %d calls", docEmbed.CallCount)
		}
	})

	t.Run("Given code type item When Update called Then uses code embedder", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "code",
		}

		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "code",
			Content: "func main() {}",
		}
		err := engine.Update(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if codeEmbed.CallCount != 1 {
			t.Errorf("expected code embedder to be called, got %d calls", codeEmbed.CallCount)
		}
	})

	t.Run("Given failure type item When Update called Then uses code embedder", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "failure",
		}

		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "failure",
			Content: "Error message",
		}
		err := engine.Update(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if codeEmbed.CallCount != 1 {
			t.Errorf("expected code embedder to be called for failure type")
		}
	})

	t.Run("Given embedding fails When Update called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "pattern",
		}

		codeEmbed := NewMockCodeEmbedder()
		codeEmbed.FailOnCall = 1

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when embedding fails")
		}
		if !errors.Is(err, ErrMockEmbedding) {
			t.Errorf("expected ErrMockEmbedding, got: %v", err)
		}
	})

	t.Run("Given doc embedding fails When Update called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "decision",
		}

		docEmbed := NewMockDocEmbedder()
		docEmbed.FailOnCall = 1

		engine := &SearchEngine{
			metadata: metaStore,
			openai:   docEmbed,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "decision",
			Content: "Content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when doc embedding fails")
		}
	})

	t.Run("Given Qdrant upsert fails When Update called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "pattern",
		}

		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		vectorStore.FailOnUpsert = 1

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when Qdrant upsert fails")
		}
	})

	t.Run("Given metadata save fails When Update called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:   "item-1",
			Type: "pattern",
		}
		metaStore.FailOnSave = 1

		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			voyage:   codeEmbed,
			qdrant:   vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Update(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when metadata save fails")
		}
	})
}

// =============================================================================
// Test: Delete
// =============================================================================

func TestSearchEngine_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("Given item exists When Delete called Then removes from both stores", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:    "item-1",
			Type:  "pattern",
			Title: "To Delete",
		}

		vectorStore := NewMockVectorStorage()
		vectorStore.Items["item-1"] = &Item{ID: "item-1"}

		engine := &SearchEngine{
			metadata: metaStore,
			qdrant:   vectorStore,
		}

		// When
		err := engine.Delete(ctx, "item-1")

		// Then
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if _, exists := metaStore.Items["item-1"]; exists {
			t.Error("expected item to be removed from metadata store")
		}
		if _, exists := vectorStore.Items["item-1"]; exists {
			t.Error("expected item to be removed from vector store")
		}
	})

	t.Run("Given item does not exist When Delete called Then succeeds silently", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			qdrant:   vectorStore,
		}

		// When
		err := engine.Delete(ctx, "nonexistent")

		// Then
		// Delete should not fail for nonexistent items (idempotent)
		if err != nil {
			t.Fatalf("Delete should succeed for nonexistent item: %v", err)
		}
	})

	t.Run("Given metadata delete fails When Delete called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.FailOnDelete = true
		vectorStore := NewMockVectorStorage()

		engine := &SearchEngine{
			metadata: metaStore,
			qdrant:   vectorStore,
		}

		// When
		err := engine.Delete(ctx, "item-1")

		// Then
		if err == nil {
			t.Fatal("expected error when metadata delete fails")
		}
	})

	t.Run("Given Qdrant delete fails When Delete called Then still deletes from metadata", func(t *testing.T) {
		// Given - The Delete method ignores Qdrant errors
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{ID: "item-1"}

		vectorStore := NewMockVectorStorage()
		vectorStore.FailOnSearch = true // This triggers error path

		engine := &SearchEngine{
			metadata: metaStore,
			qdrant:   vectorStore,
		}

		// When
		err := engine.Delete(ctx, "item-1")

		// Then - should succeed because Qdrant errors are ignored
		if err != nil {
			t.Fatalf("Delete should succeed even if Qdrant fails: %v", err)
		}
		if _, exists := metaStore.Items["item-1"]; exists {
			t.Error("expected item to be removed from metadata store")
		}
	})
}

// =============================================================================
// Test: Stats
// =============================================================================

func TestSearchEngine_Stats(t *testing.T) {
	ctx := context.Background()

	t.Run("Given items of various types When Stats called Then returns correct counts", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["p1"] = &storage.ItemRecord{ID: "p1", Type: "pattern"}
		metaStore.Items["p2"] = &storage.ItemRecord{ID: "p2", Type: "pattern"}
		metaStore.Items["p3"] = &storage.ItemRecord{ID: "p3", Type: "pattern"}
		metaStore.Items["f1"] = &storage.ItemRecord{ID: "f1", Type: "failure"}
		metaStore.Items["f2"] = &storage.ItemRecord{ID: "f2", Type: "failure"}
		metaStore.Items["d1"] = &storage.ItemRecord{ID: "d1", Type: "decision"}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		stats, err := engine.Stats(ctx)

		// Then
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}
		if stats["pattern"] != 3 {
			t.Errorf("expected 3 patterns, got %d", stats["pattern"])
		}
		if stats["failure"] != 2 {
			t.Errorf("expected 2 failures, got %d", stats["failure"])
		}
		if stats["decision"] != 1 {
			t.Errorf("expected 1 decision, got %d", stats["decision"])
		}
	})

	t.Run("Given no items When Stats called Then returns empty map", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		stats, err := engine.Stats(ctx)

		// Then
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}
		if len(stats) != 0 {
			t.Errorf("expected empty stats, got %v", stats)
		}
	})

	t.Run("Given metadata store fails When Stats called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.FailOnStats = true

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		_, err := engine.Stats(ctx)

		// Then
		if err == nil {
			t.Fatal("expected error when metadata store fails")
		}
	})
}

// =============================================================================
// Test: Search
// =============================================================================

func TestSearchEngine_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("Given items exist When Search called Then returns results with scores", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		vectorStore.Items["item-1"] = &Item{
			ID:      "item-1",
			Type:    "pattern",
			Title:   "Error Handling Pattern",
			Content: "Always check errors",
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		results, err := engine.Search(ctx, SearchRequest{
			Query: "error handling",
			Limit: 10,
		})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if len(results) > 0 && results[0].Score == 0 {
			t.Error("expected non-zero score")
		}
	})

	t.Run("Given request with type filter When Search called Then passes filter to storage", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		var capturedParams storage.SearchParams
		vectorStore.SearchFunc = func(ctx context.Context, params storage.SearchParams) ([]storage.SearchCandidate, error) {
			capturedParams = params
			return nil, nil
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		_, err := engine.Search(ctx, SearchRequest{
			Query: "test",
			Types: []string{"pattern", "failure"},
			Limit: 10,
		})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(capturedParams.Types) != 2 {
			t.Errorf("expected 2 types in params, got %d", len(capturedParams.Types))
		}
	})

	t.Run("Given request with scope filter When Search called Then passes filter to storage", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		var capturedParams storage.SearchParams
		vectorStore.SearchFunc = func(ctx context.Context, params storage.SearchParams) ([]storage.SearchCandidate, error) {
			capturedParams = params
			return nil, nil
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		_, err := engine.Search(ctx, SearchRequest{
			Query: "test",
			Scope: "global",
			Limit: 10,
		})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if capturedParams.Scope != "global" {
			t.Errorf("expected scope 'global' in params, got '%s'", capturedParams.Scope)
		}
	})

	t.Run("Given default limit When Search called without limit Then uses default of 10", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		// Add more items than default limit
		for i := 0; i < 15; i++ {
			id := "item-" + string(rune('a'+i))
			vectorStore.Items[id] = &Item{
				ID:      id,
				Type:    "pattern",
				Content: "Content",
			}
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		results, err := engine.Search(ctx, SearchRequest{
			Query: "test",
			Limit: 0, // Should default to 10
		})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) > 10 {
			t.Errorf("expected at most 10 results (default), got %d", len(results))
		}
	})

	t.Run("Given embedding fails When Search called Then returns error", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		codeEmbed.QueryFunc = func(ctx context.Context, query string) ([]float32, error) {
			return nil, ErrMockEmbedding
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
		}

		// When
		_, err := engine.Search(ctx, SearchRequest{Query: "test"})

		// Then
		if err == nil {
			t.Fatal("expected error when embedding fails")
		}
	})

	t.Run("Given Qdrant search fails When Search called Then returns error", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		vectorStore.FailOnSearch = true

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		_, err := engine.Search(ctx, SearchRequest{Query: "test"})

		// Then
		if err == nil {
			t.Fatal("expected error when Qdrant search fails")
		}
	})

	t.Run("Given no results When Search called Then returns empty slice", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		// No items in store

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		results, err := engine.Search(ctx, SearchRequest{Query: "test"})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("Given more results than limit When Search called Then truncates to limit", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		for i := 0; i < 20; i++ {
			id := "item-" + string(rune('a'+i))
			vectorStore.Items[id] = &Item{ID: id, Type: "pattern"}
		}

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		results, err := engine.Search(ctx, SearchRequest{
			Query: "test",
			Limit: 5,
		})

		// Then
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) > 5 {
			t.Errorf("expected at most 5 results, got %d", len(results))
		}
	})
}

// =============================================================================
// Test: Get
// =============================================================================

func TestSearchEngine_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("Given item exists When Get called Then returns item", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:      "item-1",
			Type:    "pattern",
			Title:   "Test Pattern",
			Content: "Content here",
			Tags:    []string{"go", "testing"},
			Scope:   "project",
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		item, err := engine.Get(ctx, "item-1")

		// Then
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if item == nil {
			t.Fatal("expected non-nil item")
		}
		if item.ID != "item-1" {
			t.Errorf("expected ID 'item-1', got '%s'", item.ID)
		}
		if item.Title != "Test Pattern" {
			t.Errorf("expected Title 'Test Pattern', got '%s'", item.Title)
		}
		if item.Type != "pattern" {
			t.Errorf("expected Type 'pattern', got '%s'", item.Type)
		}
	})

	t.Run("Given item exists with all fields When Get called Then returns complete item", func(t *testing.T) {
		// Given
		now := time.Now()
		metaStore := NewMockMetadataStorage()
		metaStore.Items["item-1"] = &storage.ItemRecord{
			ID:        "item-1",
			Type:      "pattern",
			Title:     "Test Pattern",
			Content:   "Content here",
			Tags:      []string{"go", "testing"},
			Scope:     "project",
			Source:    "/path/to/file.go",
			Metadata:  map[string]any{"key": "value"},
			CreatedAt: now,
			UpdatedAt: now,
		}

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		item, err := engine.Get(ctx, "item-1")

		// Then
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if item.Source != "/path/to/file.go" {
			t.Errorf("expected Source '/path/to/file.go', got '%s'", item.Source)
		}
		if len(item.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(item.Tags))
		}
	})

	t.Run("Given item does not exist When Get called Then returns error", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		_, err := engine.Get(ctx, "nonexistent")

		// Then
		if err == nil {
			t.Fatal("expected error for nonexistent item")
		}
	})
}

// =============================================================================
// Test: Add
// =============================================================================

func TestSearchEngine_Add(t *testing.T) {
	ctx := context.Background()

	t.Run("Given valid pattern item When Add called Then stores in both Qdrant and metadata", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			voyage:   codeEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "new-item",
			Type:    "pattern",
			Title:   "New Pattern",
			Content: "Pattern content",
			Scope:   "project",
		}
		err := engine.Add(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Add failed: %v", err)
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

	t.Run("Given valid failure item When Add called Then uses code embedder", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			voyage:   codeEmbed,
			openai:   docEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "failure-1",
			Type:    "failure",
			Content: "Failure content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if codeEmbed.CallCount != 1 {
			t.Errorf("expected code embedder to be called")
		}
		if docEmbed.CallCount != 0 {
			t.Errorf("expected doc embedder NOT to be called")
		}
	})

	t.Run("Given valid code item When Add called Then uses code embedder", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			voyage:   codeEmbed,
			openai:   docEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "code-1",
			Type:    "code",
			Content: "func main() {}",
		}
		err := engine.Add(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if codeEmbed.CallCount != 1 {
			t.Errorf("expected code embedder to be called")
		}
	})

	t.Run("Given valid decision item When Add called Then uses doc embedder", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			voyage:   codeEmbed,
			openai:   docEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "decision-1",
			Type:    "decision",
			Content: "Decision content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if docEmbed.CallCount != 1 {
			t.Errorf("expected doc embedder to be called")
		}
		if codeEmbed.CallCount != 0 {
			t.Errorf("expected code embedder NOT to be called")
		}
	})

	t.Run("Given valid context item When Add called Then uses doc embedder", func(t *testing.T) {
		// Given
		docEmbed := NewMockDocEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			openai:   docEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "context-1",
			Type:    "context",
			Content: "Context content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if docEmbed.CallCount != 1 {
			t.Errorf("expected doc embedder to be called for context type")
		}
	})

	t.Run("Given code embedding fails When Add called Then returns error", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		codeEmbed.FailOnCall = 1

		engine := &SearchEngine{
			voyage: codeEmbed,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when embedding fails")
		}
	})

	t.Run("Given doc embedding fails When Add called Then returns error", func(t *testing.T) {
		// Given
		docEmbed := NewMockDocEmbedder()
		docEmbed.FailOnCall = 1

		engine := &SearchEngine{
			openai: docEmbed,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "decision",
			Content: "Content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when embedding fails")
		}
	})

	t.Run("Given Qdrant upsert fails When Add called Then returns error", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		vectorStore.FailOnUpsert = 1

		engine := &SearchEngine{
			voyage: codeEmbed,
			qdrant: vectorStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when Qdrant upsert fails")
		}
	})

	t.Run("Given metadata save fails When Add called Then returns error", func(t *testing.T) {
		// Given
		codeEmbed := NewMockCodeEmbedder()
		vectorStore := NewMockVectorStorage()
		metaStore := NewMockMetadataStorage()
		metaStore.FailOnSave = 1

		engine := &SearchEngine{
			voyage:   codeEmbed,
			qdrant:   vectorStore,
			metadata: metaStore,
		}

		// When
		item := &Item{
			ID:      "item-1",
			Type:    "pattern",
			Content: "Content",
		}
		err := engine.Add(ctx, item)

		// Then
		if err == nil {
			t.Fatal("expected error when metadata save fails")
		}
	})
}

// =============================================================================
// Test: RecordFeedback
// =============================================================================

func TestSearchEngine_RecordFeedback(t *testing.T) {
	t.Run("Given valid feedback When RecordFeedback called Then stores feedback", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		feedback := &Feedback{
			ItemID:    "item-1",
			SessionID: "session-1",
			Useful:    true,
			Context:   "Helped solve the issue",
			Timestamp: time.Now(),
		}
		err := engine.RecordFeedback(feedback)

		// Then
		if err != nil {
			t.Fatalf("RecordFeedback failed: %v", err)
		}
		if len(metaStore.Feedback) != 1 {
			t.Errorf("expected 1 feedback record, got %d", len(metaStore.Feedback))
		}
	})

	t.Run("Given feedback with negative rating When RecordFeedback called Then stores feedback", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		feedback := &Feedback{
			ItemID:    "item-1",
			SessionID: "session-1",
			Useful:    false,
			Context:   "Not relevant",
			Timestamp: time.Now(),
		}
		err := engine.RecordFeedback(feedback)

		// Then
		if err != nil {
			t.Fatalf("RecordFeedback failed: %v", err)
		}
		if metaStore.Feedback[0].Useful {
			t.Error("expected feedback to be marked as not useful")
		}
	})
}

// =============================================================================
// Test: LogFlightRecorder
// =============================================================================

func TestSearchEngine_LogFlightRecorder(t *testing.T) {
	t.Run("Given valid entry When LogFlightRecorder called Then stores entry", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		entry := &FlightRecorderEntry{
			ID:        "entry-1",
			SessionID: "session-1",
			Timestamp: time.Now(),
			Type:      "decision",
			Content:   "Decided to use pattern X",
			Rationale: "Better performance",
		}
		err := engine.LogFlightRecorder(entry)

		// Then
		if err != nil {
			t.Fatalf("LogFlightRecorder failed: %v", err)
		}
		if len(metaStore.FlightRecorder) != 1 {
			t.Errorf("expected 1 flight recorder entry, got %d", len(metaStore.FlightRecorder))
		}
	})

	t.Run("Given entry with metadata When LogFlightRecorder called Then stores metadata", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		entry := &FlightRecorderEntry{
			ID:        "entry-1",
			SessionID: "session-1",
			Timestamp: time.Now(),
			Type:      "observation",
			Content:   "Found pattern",
			Metadata:  map[string]any{"tag": "parallel-discovery"},
		}
		err := engine.LogFlightRecorder(entry)

		// Then
		if err != nil {
			t.Fatalf("LogFlightRecorder failed: %v", err)
		}
		if metaStore.FlightRecorder[0].Metadata == nil {
			t.Error("expected metadata to be stored")
		}
	})
}

// =============================================================================
// Test: Close
// =============================================================================

func TestSearchEngine_Close(t *testing.T) {
	t.Run("Given engine with metadata store When Close called Then closes store", func(t *testing.T) {
		// Given
		metaStore := NewMockMetadataStorage()

		engine := &SearchEngine{
			metadata: metaStore,
		}

		// When
		err := engine.Close()

		// Then
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
		if !metaStore.Closed {
			t.Error("expected metadata store to be closed")
		}
	})

	t.Run("Given engine with nil metadata store When Close called Then succeeds", func(t *testing.T) {
		// Given
		engine := &SearchEngine{
			metadata: nil,
		}

		// When
		err := engine.Close()

		// Then
		if err != nil {
			t.Fatalf("Close should not fail with nil metadata: %v", err)
		}
	})
}

// =============================================================================
// Test: Helper Functions
// =============================================================================

func TestToDocuments(t *testing.T) {
	t.Run("Given search results When toDocuments called Then converts correctly", func(t *testing.T) {
		// Given
		results := []SearchResult{
			{Item: Item{ID: "id-1", Content: "content-1"}},
			{Item: Item{ID: "id-2", Content: "content-2"}},
		}

		// When
		docs := toDocuments(results)

		// Then
		if len(docs) != 2 {
			t.Errorf("expected 2 documents, got %d", len(docs))
		}
		if docs[0].ID != "id-1" {
			t.Errorf("expected ID 'id-1', got '%s'", docs[0].ID)
		}
		if docs[0].Content != "content-1" {
			t.Errorf("expected Content 'content-1', got '%s'", docs[0].Content)
		}
	})

	t.Run("Given empty results When toDocuments called Then returns empty slice", func(t *testing.T) {
		// When
		docs := toDocuments([]SearchResult{})

		// Then
		if len(docs) != 0 {
			t.Errorf("expected 0 documents, got %d", len(docs))
		}
	})
}

func TestItemFromRecord(t *testing.T) {
	t.Run("Given complete record When itemFromRecord called Then converts all fields", func(t *testing.T) {
		// Given
		now := time.Now()
		record := &storage.ItemRecord{
			ID:        "id-1",
			Type:      "pattern",
			Title:     "Title",
			Content:   "Content",
			Tags:      []string{"tag1", "tag2"},
			Scope:     "project",
			Source:    "/path/to/file",
			Metadata:  map[string]any{"key": "value"},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// When
		item := itemFromRecord(record)

		// Then
		if item.ID != "id-1" {
			t.Errorf("ID mismatch")
		}
		if item.Type != "pattern" {
			t.Errorf("Type mismatch")
		}
		if item.Title != "Title" {
			t.Errorf("Title mismatch")
		}
		if item.Content != "Content" {
			t.Errorf("Content mismatch")
		}
		if len(item.Tags) != 2 {
			t.Errorf("Tags count mismatch")
		}
		if item.Scope != "project" {
			t.Errorf("Scope mismatch")
		}
		if item.Source != "/path/to/file" {
			t.Errorf("Source mismatch")
		}
	})
}

func TestItemToRecord(t *testing.T) {
	t.Run("Given complete item When itemToRecord called Then converts all fields", func(t *testing.T) {
		// Given
		now := time.Now()
		item := &Item{
			ID:        "id-1",
			Type:      "pattern",
			Title:     "Title",
			Content:   "Content",
			Tags:      []string{"tag1", "tag2"},
			Scope:     "project",
			Source:    "/path/to/file",
			Metadata:  map[string]any{"key": "value"},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// When
		record := itemToRecord(item)

		// Then
		if record.ID != "id-1" {
			t.Errorf("ID mismatch")
		}
		if record.Type != "pattern" {
			t.Errorf("Type mismatch")
		}
		if record.Title != "Title" {
			t.Errorf("Title mismatch")
		}
	})
}

// =============================================================================
// Helper functions
// =============================================================================

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
