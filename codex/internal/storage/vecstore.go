package storage

import (
	"container/heap"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
)

// VecStore provides brute-force vector search backed by SQLite BLOBs.
// Vectors are loaded into memory for fast cosine similarity computation.
// At <10K documents this is sub-millisecond and returns exact (not approximate) results.
type VecStore struct {
	db *sql.DB

	mu      sync.RWMutex
	vectors map[string][]float32 // item_id -> normalized embedding
}

// ScoredResult pairs an item ID with a similarity score.
type ScoredResult struct {
	ID    string
	Score float64
}

// NewVecStore creates a vector store using the given SQLite database.
// It creates the vectors table if needed and loads existing vectors into memory.
func NewVecStore(db *sql.DB) (*VecStore, error) {
	vs := &VecStore{
		db:      db,
		vectors: make(map[string][]float32),
	}

	if err := vs.migrate(); err != nil {
		return nil, fmt.Errorf("vecstore migrate: %w", err)
	}

	if err := vs.loadAll(); err != nil {
		return nil, fmt.Errorf("vecstore load: %w", err)
	}

	return vs, nil
}

func (vs *VecStore) migrate() error {
	_, err := vs.db.Exec(`
		CREATE TABLE IF NOT EXISTS vectors (
			item_id    TEXT PRIMARY KEY,
			embedding  BLOB NOT NULL,
			dimensions INTEGER NOT NULL
		)
	`)
	return err
}

func (vs *VecStore) loadAll() error {
	rows, err := vs.db.Query("SELECT item_id, embedding, dimensions FROM vectors")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var blob []byte
		var dims int

		if err := rows.Scan(&id, &blob, &dims); err != nil {
			return err
		}

		vec := blobToFloat32(blob, dims)
		vs.vectors[id] = vec
	}
	return rows.Err()
}

// Upsert stores a pre-normalized vector for the given item ID.
// The vector is normalized on insert so dot product equals cosine similarity.
func (vs *VecStore) Upsert(ctx context.Context, itemID string, vector []float32) error {
	normalized := normalize(vector)
	blob := float32ToBlob(normalized)

	vs.mu.Lock()
	defer vs.mu.Unlock()

	_, err := vs.db.ExecContext(ctx, `
		INSERT INTO vectors (item_id, embedding, dimensions)
		VALUES (?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET
			embedding=excluded.embedding, dimensions=excluded.dimensions
	`, itemID, blob, len(normalized))
	if err != nil {
		return err
	}

	vs.vectors[itemID] = normalized
	return nil
}

// Search returns the top-K items by cosine similarity to the query vector.
// Uses a min-heap to efficiently track only the top-K results.
func (vs *VecStore) Search(ctx context.Context, queryVec []float32, limit int) []ScoredResult {
	if limit <= 0 {
		limit = 10
	}
	normalizedQuery := normalize(queryVec)

	vs.mu.RLock()
	h := &minHeap{}
	heap.Init(h)
	for id, vec := range vs.vectors {
		if len(vec) != len(normalizedQuery) {
			continue
		}
		score := dotProduct(normalizedQuery, vec)
		if h.Len() < limit {
			heap.Push(h, ScoredResult{ID: id, Score: score})
		} else if score > (*h)[0].Score {
			(*h)[0] = ScoredResult{ID: id, Score: score}
			heap.Fix(h, 0)
		}
	}
	vs.mu.RUnlock()

	// Extract results in descending score order
	results := make([]ScoredResult, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(ScoredResult)
	}
	return results
}

// minHeap implements heap.Interface for top-K selection (min at root).
type minHeap []ScoredResult

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool   { return h[i].Score < h[j].Score }
func (h minHeap) Swap(i, j int)        { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)  { *h = append(*h, x.(ScoredResult)) }
func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// Delete removes a vector by item ID.
func (vs *VecStore) Delete(ctx context.Context, itemID string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	_, err := vs.db.ExecContext(ctx, "DELETE FROM vectors WHERE item_id = ?", itemID)
	if err != nil {
		return err
	}

	delete(vs.vectors, itemID)
	return nil
}

// Count returns the number of stored vectors.
func (vs *VecStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.vectors)
}

// --- math helpers ---

func normalize(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		out := make([]float32, len(v))
		return out
	}

	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

func dotProduct(a, b []float32) float64 {
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// --- serialization helpers ---

func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func blobToFloat32(b []byte, dims int) []float32 {
	v := make([]float32, dims)
	for i := 0; i < dims && i*4+4 <= len(b); i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
