package eval

// JudgeMetrics holds per-query metrics for the LLM judge evaluation.
type JudgeMetrics struct {
	QueryID         string  `json:"query_id"`
	Query           string  `json:"query"`
	Category        string  `json:"category"`
	JudgePrecision  float64 `json:"judge_precision"`  // |judged ∩ true| / |judged|
	JudgeRecall     float64 `json:"judge_recall"`      // |judged ∩ true ∩ retrieved| / |true ∩ retrieved|
	JudgeF1         float64 `json:"judge_f1"`
	FilteringRate   float64 `json:"filtering_rate"`    // 1 - |judged| / |retrieved|
	RawPrecisionAt5 float64 `json:"raw_precision_at5"` // baseline from raw retrieval
	Improvement     float64 `json:"improvement"`       // judge precision - raw precision
}

// JudgeCategoryMetrics holds aggregated metrics for a query category.
type JudgeCategoryMetrics struct {
	AvgJudgePrecision float64 `json:"avg_judge_precision"`
	AvgJudgeRecall    float64 `json:"avg_judge_recall"`
	AvgJudgeF1        float64 `json:"avg_judge_f1"`
	AvgFilteringRate  float64 `json:"avg_filtering_rate"`
	AvgImprovement    float64 `json:"avg_improvement"`
	Count             int     `json:"count"`
}

// JudgeSummary holds aggregated judge evaluation results.
type JudgeSummary struct {
	AvgJudgePrecision float64                       `json:"avg_judge_precision"`
	AvgJudgeRecall    float64                       `json:"avg_judge_recall"`
	AvgJudgeF1        float64                       `json:"avg_judge_f1"`
	AvgFilteringRate  float64                       `json:"avg_filtering_rate"`
	AvgImprovement    float64                       `json:"avg_improvement"`
	ByCategory        map[string]*JudgeCategoryMetrics `json:"by_category"`
	PerQuery          []JudgeMetrics                `json:"per_query"`
}

// computeJudgeMetrics computes judge precision, recall, F1 for a single query.
func computeJudgeMetrics(judgedIDs, retrievedIDs, relevantIDs []string) (precision, recall, f1, filteringRate float64) {
	relSet := toSet(relevantIDs)
	retSet := toSet(retrievedIDs)

	// Judge precision: |judged ∩ relevant| / |judged|
	if len(judgedIDs) == 0 {
		precision = 1.0 // no false positives if nothing selected
	} else {
		hits := 0
		for _, id := range judgedIDs {
			if relSet[id] {
				hits++
			}
		}
		precision = float64(hits) / float64(len(judgedIDs))
	}

	// Judge recall: |judged ∩ relevant ∩ retrieved| / |relevant ∩ retrieved|
	// Only counts relevant docs that were actually retrieved (judge can't find what wasn't returned)
	relevantRetrieved := 0
	for _, id := range relevantIDs {
		if retSet[id] {
			relevantRetrieved++
		}
	}
	if relevantRetrieved == 0 {
		recall = 1.0 // nothing to recall
	} else {
		judgedSet := toSet(judgedIDs)
		hits := 0
		for _, id := range relevantIDs {
			if retSet[id] && judgedSet[id] {
				hits++
			}
		}
		recall = float64(hits) / float64(relevantRetrieved)
	}

	// F1
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	// Filtering rate: 1 - |judged| / |retrieved|
	if len(retrievedIDs) > 0 {
		filteringRate = 1.0 - float64(len(judgedIDs))/float64(len(retrievedIDs))
	}

	return
}

// aggregateJudgeMetrics computes a JudgeSummary from per-query metrics.
func aggregateJudgeMetrics(perQuery []JudgeMetrics) *JudgeSummary {
	s := &JudgeSummary{
		ByCategory: make(map[string]*JudgeCategoryMetrics),
		PerQuery:   perQuery,
	}

	if len(perQuery) == 0 {
		return s
	}

	for _, m := range perQuery {
		s.AvgJudgePrecision += m.JudgePrecision
		s.AvgJudgeRecall += m.JudgeRecall
		s.AvgJudgeF1 += m.JudgeF1
		s.AvgFilteringRate += m.FilteringRate
		s.AvgImprovement += m.Improvement

		cat := m.Category
		if _, ok := s.ByCategory[cat]; !ok {
			s.ByCategory[cat] = &JudgeCategoryMetrics{}
		}
		c := s.ByCategory[cat]
		c.AvgJudgePrecision += m.JudgePrecision
		c.AvgJudgeRecall += m.JudgeRecall
		c.AvgJudgeF1 += m.JudgeF1
		c.AvgFilteringRate += m.FilteringRate
		c.AvgImprovement += m.Improvement
		c.Count++
	}

	n := float64(len(perQuery))
	s.AvgJudgePrecision /= n
	s.AvgJudgeRecall /= n
	s.AvgJudgeF1 /= n
	s.AvgFilteringRate /= n
	s.AvgImprovement /= n

	for _, c := range s.ByCategory {
		cn := float64(c.Count)
		c.AvgJudgePrecision /= cn
		c.AvgJudgeRecall /= cn
		c.AvgJudgeF1 /= cn
		c.AvgFilteringRate /= cn
		c.AvgImprovement /= cn
	}

	return s
}
