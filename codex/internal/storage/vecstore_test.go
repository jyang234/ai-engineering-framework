package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func createTestVecStore(t *testing.T) (*VecStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "vecstore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open DB: %v", err)
	}

	vs, err := NewVecStore(db)
	if err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create VecStore: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return vs, cleanup
}

func TestVecStore_UpsertAndSearch(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	similar := []float32{1.0, 0.0, 0.0}
	dissimilar := []float32{0.0, 1.0, 0.0}
	query := []float32{0.9, 0.1, 0.0}

	if err := vs.Upsert(ctx, "similar", similar); err != nil {
		t.Fatalf("Upsert similar: %v", err)
	}
	if err := vs.Upsert(ctx, "dissimilar", dissimilar); err != nil {
		t.Fatalf("Upsert dissimilar: %v", err)
	}

	results, _ := vs.Search(ctx, query, 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].ID != "similar" {
		t.Errorf("expected 'similar' first, got '%s'", results[0].ID)
	}
	if results[1].ID != "dissimilar" {
		t.Errorf("expected 'dissimilar' second, got '%s'", results[1].ID)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected similar score > dissimilar score, got %f <= %f",
			results[0].Score, results[1].Score)
	}
}

func TestVecStore_CosineSimilarityCorrectness(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	vec := []float32{1.0, 2.0, 3.0}
	if err := vs.Upsert(ctx, "same", vec); err != nil {
		t.Fatal(err)
	}

	results, _ := vs.Search(ctx, vec, 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if math.Abs(results[0].Score-1.0) > 0.001 {
		t.Errorf("expected score ~1.0 for identical vectors, got %f", results[0].Score)
	}
}

func TestVecStore_Delete(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	vec := []float32{1.0, 0.0, 0.0}
	vs.Upsert(ctx, "item1", vec)
	vs.Upsert(ctx, "item2", vec)

	if vs.Count() != 2 {
		t.Fatalf("expected count 2, got %d", vs.Count())
	}

	if err := vs.Delete(ctx, "item1"); err != nil {
		t.Fatal(err)
	}

	if vs.Count() != 1 {
		t.Errorf("expected count 1 after delete, got %d", vs.Count())
	}

	results, _ := vs.Search(ctx, vec, 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result after delete, got %d", len(results))
	}
	if results[0].ID != "item2" {
		t.Errorf("expected remaining item to be 'item2', got '%s'", results[0].ID)
	}
}

func TestVecStore_UpsertOverwrite(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	vs.Upsert(ctx, "item", []float32{1.0, 0.0, 0.0})
	vs.Upsert(ctx, "item", []float32{0.0, 1.0, 0.0})

	if vs.Count() != 1 {
		t.Errorf("expected count 1 after upsert, got %d", vs.Count())
	}

	results, _ := vs.Search(ctx, []float32{0.0, 1.0, 0.0}, 1)
	if math.Abs(results[0].Score-1.0) > 0.001 {
		t.Errorf("expected score ~1.0 for updated vector, got %f", results[0].Score)
	}
}

func TestVecStore_SearchLimit(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		vec := make([]float32, 3)
		vec[i%3] = 1.0
		vs.Upsert(ctx, fmt.Sprintf("item-%d", i), vec)
	}

	results, _ := vs.Search(ctx, []float32{1.0, 0.0, 0.0}, 3)
	if len(results) != 3 {
		t.Errorf("expected 3 results with limit, got %d", len(results))
	}
}

func TestVecStore_EmptySearch(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()
	results, _ := vs.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}
}

func TestVecStore_DimensionMismatch(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	vs.Upsert(ctx, "item", []float32{1.0, 0.0, 0.0})

	results, _ := vs.Search(ctx, []float32{1.0, 0.0, 0.0, 0.0, 0.0}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for dimension mismatch, got %d", len(results))
	}
}

func TestVecStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vecstore-persist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Write data
	{
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		vs, err := NewVecStore(db)
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := vs.Upsert(ctx, "item", []float32{1.0, 2.0, 3.0}); err != nil {
			db.Close()
			t.Fatal(err)
		}
		db.Close()
	}

	// Reopen and verify
	{
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		vs, err := NewVecStore(db)
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		defer db.Close()

		if vs.Count() != 1 {
			t.Errorf("expected 1 item after reopen, got %d", vs.Count())
		}

		results, _ := vs.Search(ctx, []float32{1.0, 2.0, 3.0}, 1)
		if len(results) != 1 || results[0].ID != "item" {
			t.Errorf("expected to find 'item' after reopen")
		}
		if math.Abs(results[0].Score-1.0) > 0.001 {
			t.Errorf("expected score ~1.0 after reopen, got %f", results[0].Score)
		}
	}
}

// =============================================================================
// Cross-process cache invalidation tests (Change 5)
// =============================================================================

func TestVecStore_DetectsExternalInsert(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vecstore-xproc-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Open two VecStore instances on the same DB (simulating two processes)
	dbA, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	defer dbA.Close()

	storeA, err := NewVecStore(dbA)
	if err != nil {
		t.Fatal(err)
	}

	dbB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	defer dbB.Close()

	storeB, err := NewVecStore(dbB)
	if err != nil {
		t.Fatal(err)
	}

	// Insert via store A
	if err := storeA.Upsert(ctx, "ext-item", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatalf("Upsert via A: %v", err)
	}

	// Force stale check on store B by resetting lastCheckTime
	storeB.mu.Lock()
	storeB.lastCheckTime = storeB.lastCheckTime.Add(-2 * vectorCacheCheckInterval)
	storeB.mu.Unlock()

	// Search on store B — should detect the external insert and find the item
	results, err := storeB.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)
	if err != nil {
		t.Fatalf("Search on B: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected store B to find 1 result after external insert, got %d", len(results))
	}
}

func TestVecStore_DetectsExternalDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vecstore-xproc-del-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Shared DB
	dbA, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	defer dbA.Close()

	storeA, err := NewVecStore(dbA)
	if err != nil {
		t.Fatal(err)
	}

	// Insert an item that both stores will see at init
	if err := storeA.Upsert(ctx, "will-delete", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatal(err)
	}

	dbB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	defer dbB.Close()

	storeB, err := NewVecStore(dbB)
	if err != nil {
		t.Fatal(err)
	}

	// Verify B can find the item
	results, _ := storeB.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	// Delete via store A
	if err := storeA.Delete(ctx, "will-delete"); err != nil {
		t.Fatal(err)
	}

	// Force stale check on B
	storeB.mu.Lock()
	storeB.lastCheckTime = storeB.lastCheckTime.Add(-2 * vectorCacheCheckInterval)
	storeB.mu.Unlock()

	// Search on B — should detect the external delete
	results, _ = storeB.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results after external delete, got %d", len(results))
	}
}

func TestVecStore_RateLimitsCheck(t *testing.T) {
	vs, cleanup := createTestVecStore(t)
	defer cleanup()

	ctx := context.Background()

	// Insert an item so there's something to search
	vs.Upsert(ctx, "item", []float32{1.0, 0.0, 0.0})

	// First search will do the stale check (lastCheckTime is zero)
	vs.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)

	// Record the check time
	vs.mu.RLock()
	checkAfterFirst := vs.lastCheckTime
	vs.mu.RUnlock()

	// Second search immediately after — should be rate-limited, not update lastCheckTime
	vs.Search(ctx, []float32{1.0, 0.0, 0.0}, 10)

	vs.mu.RLock()
	checkAfterSecond := vs.lastCheckTime
	vs.mu.RUnlock()

	if !checkAfterFirst.Equal(checkAfterSecond) {
		t.Errorf("expected rate-limited check (same timestamp), but got different: %v vs %v",
			checkAfterFirst, checkAfterSecond)
	}
}

func TestNormalize(t *testing.T) {
	v := normalize([]float32{3.0, 4.0})
	if math.Abs(float64(v[0])-0.6) > 0.001 || math.Abs(float64(v[1])-0.8) > 0.001 {
		t.Errorf("normalize([3,4]) = [%f, %f], want [0.6, 0.8]", v[0], v[1])
	}
}

func TestNormalize_ZeroVector(t *testing.T) {
	v := normalize([]float32{0.0, 0.0, 0.0})
	for i, x := range v {
		if x != 0.0 {
			t.Errorf("normalize zero vector [%d] = %f, want 0", i, x)
		}
	}
}

func TestDotProduct(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{4.0, 5.0, 6.0}
	got := dotProduct(a, b)
	want := 32.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("dotProduct = %f, want %f", got, want)
	}
}

func TestFloat32BlobRoundTrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 1000.0, math.SmallestNonzeroFloat32}
	blob := float32ToBlob(original)
	restored, err := blobToFloat32(blob, len(original))
	if err != nil {
		t.Fatalf("blobToFloat32: %v", err)
	}

	for i := range original {
		if original[i] != restored[i] {
			t.Errorf("roundtrip mismatch at [%d]: %f != %f", i, original[i], restored[i])
		}
	}
}
