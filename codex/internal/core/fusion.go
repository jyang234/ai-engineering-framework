package core

import (
	"sort"

	"github.com/anthropics/aef/codex/internal/storage"
)

// reciprocalRankFusion merges vector and keyword search results using RRF.
// k is the standard RRF constant (typically 60).
// Each result's fused score is sum(1 / (k + rank)) across all result lists it appears in.
func reciprocalRankFusion(vectorResults []storage.ScoredResult, keywordResults []SearchResult, k float64) []SearchResult {
	return reciprocalRankFusionMulti([][]storage.ScoredResult{vectorResults}, keywordResults, k)
}

// reciprocalRankFusionMulti merges multiple vector result lists and keyword results using RRF.
// Each vector result list and the keyword result list contribute 1/(k+rank) to a document's score.
func reciprocalRankFusionMulti(vectorResultSets [][]storage.ScoredResult, keywordResults []SearchResult, k float64) []SearchResult {
	scores := make(map[string]float64)
	meta := make(map[string]SearchResult)

	// Score each vector result set by rank position
	for _, vectorResults := range vectorResultSets {
		for rank, r := range vectorResults {
			scores[r.ID] += 1.0 / (k + float64(rank+1))
		}
	}

	// Score keyword results by rank position
	for rank, r := range keywordResults {
		scores[r.ID] += 1.0 / (k + float64(rank+1))
		meta[r.ID] = r
	}

	// Build merged results
	var merged []SearchResult
	for id, score := range scores {
		result, ok := meta[id]
		if !ok {
			// Vector-only result; caller must hydrate metadata separately
			result = SearchResult{Item: Item{ID: id}}
		}
		result.Score = score
		merged = append(merged, result)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged
}
