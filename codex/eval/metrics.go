package eval

import "math"

// RecallAtK computes recall@K: fraction of relevant items found in the top-K results.
// retrieved is the ordered list of result IDs, relevant is the set of ground truth IDs.
func RecallAtK(retrieved []string, relevant []string, k int) float64 {
	if len(relevant) == 0 || k <= 0 {
		return 0
	}
	relSet := toSet(relevant)
	topK := retrieved
	if k < len(topK) {
		topK = topK[:k]
	}
	found := 0
	for _, id := range topK {
		if relSet[id] {
			found++
		}
	}
	return float64(found) / float64(len(relevant))
}

// PrecisionAtK computes precision@K: fraction of top-K results that are relevant.
func PrecisionAtK(retrieved []string, relevant []string, k int) float64 {
	if k <= 0 {
		return 0
	}
	relSet := toSet(relevant)
	topK := retrieved
	if k < len(topK) {
		topK = topK[:k]
	}
	if len(topK) == 0 {
		return 0
	}
	found := 0
	for _, id := range topK {
		if relSet[id] {
			found++
		}
	}
	return float64(found) / float64(len(topK))
}

// NDCG computes normalized discounted cumulative gain at K.
// The relevance of each document is determined by its position in the relevant list
// (first = most relevant). Documents not in relevant get relevance 0.
func NDCG(retrieved []string, relevant []string, k int) float64 {
	if len(relevant) == 0 || k <= 0 {
		return 0
	}
	// Build relevance scores: position 0 in relevant = highest relevance
	relScore := make(map[string]float64)
	for i, id := range relevant {
		relScore[id] = float64(len(relevant) - i)
	}

	topK := retrieved
	if k < len(topK) {
		topK = topK[:k]
	}

	// DCG
	dcg := 0.0
	for i, id := range topK {
		if score, ok := relScore[id]; ok {
			dcg += score / math.Log2(float64(i+2)) // i+2 because log2(1)=0
		}
	}

	// Ideal DCG: sorted relevant docs
	idealK := k
	if idealK > len(relevant) {
		idealK = len(relevant)
	}
	idcg := 0.0
	for i := 0; i < idealK; i++ {
		score := float64(len(relevant) - i)
		idcg += score / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// MRR computes mean reciprocal rank: 1/rank of first relevant result.
func MRR(retrieved []string, relevant []string) float64 {
	relSet := toSet(relevant)
	for i, id := range retrieved {
		if relSet[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
