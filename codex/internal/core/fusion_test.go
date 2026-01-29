package core

import (
	"math"
	"testing"

	"github.com/anthropics/aef/codex/internal/storage"
)

func TestReciprocalRankFusion_MergesResults(t *testing.T) {
	vectorResults := []storage.ScoredResult{
		{ID: "a", Score: 0.9},
		{ID: "b", Score: 0.8},
		{ID: "c", Score: 0.7},
	}

	keywordResults := []SearchResult{
		{Item: Item{ID: "b", Title: "B"}, Score: 10.0},
		{Item: Item{ID: "d", Title: "D"}, Score: 8.0},
		{Item: Item{ID: "a", Title: "A"}, Score: 6.0},
	}

	merged := reciprocalRankFusion(vectorResults, keywordResults, 60)

	// "a" and "b" appear in both lists and should have higher fused scores
	if len(merged) != 4 {
		t.Fatalf("expected 4 merged results, got %d", len(merged))
	}

	// Items in both lists should rank higher
	topTwo := map[string]bool{merged[0].ID: true, merged[1].ID: true}
	if !topTwo["a"] || !topTwo["b"] {
		t.Errorf("expected 'a' and 'b' in top 2, got %s and %s", merged[0].ID, merged[1].ID)
	}
}

func TestReciprocalRankFusion_EmptyInputs(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		merged := reciprocalRankFusion(nil, nil, 60)
		if len(merged) != 0 {
			t.Errorf("expected 0 results, got %d", len(merged))
		}
	})

	t.Run("vector only", func(t *testing.T) {
		vectorResults := []storage.ScoredResult{
			{ID: "a", Score: 0.9},
		}
		merged := reciprocalRankFusion(vectorResults, nil, 60)
		if len(merged) != 1 {
			t.Fatalf("expected 1 result, got %d", len(merged))
		}
		if merged[0].ID != "a" {
			t.Errorf("expected 'a', got '%s'", merged[0].ID)
		}
	})

	t.Run("keyword only", func(t *testing.T) {
		keywordResults := []SearchResult{
			{Item: Item{ID: "b", Title: "B"}, Score: 10.0},
		}
		merged := reciprocalRankFusion(nil, keywordResults, 60)
		if len(merged) != 1 {
			t.Fatalf("expected 1 result, got %d", len(merged))
		}
		if merged[0].ID != "b" {
			t.Errorf("expected 'b', got '%s'", merged[0].ID)
		}
	})
}

func TestReciprocalRankFusion_ScoreCalculation(t *testing.T) {
	// Single item in both lists at rank 1
	vectorResults := []storage.ScoredResult{{ID: "x", Score: 1.0}}
	keywordResults := []SearchResult{{Item: Item{ID: "x"}, Score: 1.0}}

	merged := reciprocalRankFusion(vectorResults, keywordResults, 60)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}

	// Expected score: 1/(60+1) + 1/(60+1) = 2/61
	expected := 2.0 / 61.0
	if math.Abs(merged[0].Score-expected) > 0.0001 {
		t.Errorf("expected score %f, got %f", expected, merged[0].Score)
	}
}

func TestReciprocalRankFusion_PreservesMetadata(t *testing.T) {
	keywordResults := []SearchResult{
		{Item: Item{ID: "a", Title: "Title A", Content: "Content A", Type: "pattern"}, Score: 1.0},
	}

	merged := reciprocalRankFusion(nil, keywordResults, 60)
	if merged[0].Title != "Title A" {
		t.Errorf("expected title preserved, got '%s'", merged[0].Title)
	}
	if merged[0].Content != "Content A" {
		t.Errorf("expected content preserved, got '%s'", merged[0].Content)
	}
}

func TestReciprocalRankFusionMulti_ThreeWayFusion(t *testing.T) {
	voyageResults := []storage.ScoredResult{
		{ID: "pattern-1", Score: 0.9},
		{ID: "pattern-2", Score: 0.8},
	}
	openaiResults := []storage.ScoredResult{
		{ID: "adr-1", Score: 0.95},
		{ID: "arch-1", Score: 0.85},
	}
	keywordResults := []SearchResult{
		{Item: Item{ID: "pattern-1", Title: "P1"}, Score: 10.0},
		{Item: Item{ID: "adr-1", Title: "ADR1"}, Score: 8.0},
	}

	merged := reciprocalRankFusionMulti(
		[][]storage.ScoredResult{voyageResults, openaiResults},
		keywordResults, 60,
	)

	// Should contain all 4 unique docs
	if len(merged) != 4 {
		t.Fatalf("expected 4 merged results, got %d", len(merged))
	}

	// pattern-1 and adr-1 appear in two lists each, should rank highest
	topTwo := map[string]bool{merged[0].ID: true, merged[1].ID: true}
	if !topTwo["pattern-1"] || !topTwo["adr-1"] {
		t.Errorf("expected pattern-1 and adr-1 in top 2, got %s and %s", merged[0].ID, merged[1].ID)
	}
}

func TestReciprocalRankFusionMulti_ScoreCalculation(t *testing.T) {
	// Item "x" at rank 1 in all three lists
	voyage := []storage.ScoredResult{{ID: "x", Score: 1.0}}
	openai := []storage.ScoredResult{{ID: "x", Score: 1.0}}
	kw := []SearchResult{{Item: Item{ID: "x"}, Score: 1.0}}

	merged := reciprocalRankFusionMulti([][]storage.ScoredResult{voyage, openai}, kw, 60)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}

	// Expected: 3 * 1/(60+1) = 3/61
	expected := 3.0 / 61.0
	if math.Abs(merged[0].Score-expected) > 0.0001 {
		t.Errorf("expected score %f, got %f", expected, merged[0].Score)
	}
}

func TestReciprocalRankFusion_SortedByScore(t *testing.T) {
	vectorResults := []storage.ScoredResult{
		{ID: "low", Score: 0.1},
		{ID: "high", Score: 0.9},
	}
	keywordResults := []SearchResult{
		{Item: Item{ID: "high"}, Score: 10.0},
		{Item: Item{ID: "low"}, Score: 1.0},
	}

	merged := reciprocalRankFusion(vectorResults, keywordResults, 60)

	for i := 1; i < len(merged); i++ {
		if merged[i].Score > merged[i-1].Score {
			t.Errorf("results not sorted: [%d]=%f > [%d]=%f",
				i, merged[i].Score, i-1, merged[i-1].Score)
		}
	}
}
