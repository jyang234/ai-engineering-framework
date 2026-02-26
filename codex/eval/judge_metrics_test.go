package eval

import (
	"math"
	"testing"
)

// =============================================================================
// Judge Metrics Tests — Given-When-Then
// =============================================================================

// --- computeJudgeMetrics ---

func TestComputeJudgeMetrics_GivenPerfectJudge_ThenAllMetricsOptimal(t *testing.T) {
	// Given: judged IDs exactly match the relevant IDs within the retrieved set
	judged := []string{"a", "b", "c"}
	retrieved := []string{"a", "b", "c", "d", "e"}
	relevant := []string{"a", "b", "c"}

	// When: we compute metrics
	precision, recall, f1, filteringRate := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: precision=1.0 (all judged are relevant), recall=1.0 (all relevant retrieved are judged)
	assertFloat(t, "precision", precision, 1.0)
	assertFloat(t, "recall", recall, 1.0)
	assertFloat(t, "f1", f1, 1.0)
	// Filtering rate: 1 - 3/5 = 0.4 (judge filtered out 2 of 5 retrieved)
	assertFloat(t, "filteringRate", filteringRate, 0.4)
}

func TestComputeJudgeMetrics_GivenJudgeIncludesIrrelevant_ThenPrecisionDrops(t *testing.T) {
	// Given: judged includes 2 relevant + 1 irrelevant
	judged := []string{"a", "b", "x"}
	retrieved := []string{"a", "b", "x", "y"}
	relevant := []string{"a", "b"}

	// When: we compute metrics
	precision, recall, _, _ := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: precision = 2/3 (a,b relevant out of 3 judged), recall = 2/2 = 1.0
	assertFloat(t, "precision", precision, 2.0/3.0)
	assertFloat(t, "recall", recall, 1.0)
}

func TestComputeJudgeMetrics_GivenJudgeMissesRelevant_ThenRecallDrops(t *testing.T) {
	// Given: judged misses one relevant item that was retrieved
	judged := []string{"a"}
	retrieved := []string{"a", "b", "c"}
	relevant := []string{"a", "b"}

	// When: we compute metrics
	precision, recall, _, _ := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: precision=1.0 (all judged are relevant), recall=0.5 (1 of 2 relevant-retrieved)
	assertFloat(t, "precision", precision, 1.0)
	assertFloat(t, "recall", recall, 0.5)
}

func TestComputeJudgeMetrics_GivenEmptyJudged_ThenPrecisionOneRecallZero(t *testing.T) {
	// Given: judge selected nothing
	judged := []string{}
	retrieved := []string{"a", "b"}
	relevant := []string{"a"}

	// When: we compute metrics
	precision, recall, f1, filteringRate := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: precision=1.0 (no false positives), recall=0 (missed all), f1=0
	assertFloat(t, "precision", precision, 1.0)
	assertFloat(t, "recall", recall, 0.0)
	assertFloat(t, "f1", f1, 0.0)
	// Filtering rate: 1 - 0/2 = 1.0 (everything was filtered)
	assertFloat(t, "filteringRate", filteringRate, 1.0)
}

func TestComputeJudgeMetrics_GivenNoRelevantInRetrieved_ThenRecallOne(t *testing.T) {
	// Given: no relevant items were in the retrieved set (judge can't find what's not there)
	judged := []string{"x", "y"}
	retrieved := []string{"x", "y", "z"}
	relevant := []string{"a", "b"} // none overlap with retrieved

	// When: we compute metrics
	_, recall, _, _ := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: recall=1.0 (nothing to recall → vacuously true)
	assertFloat(t, "recall", recall, 1.0)
}

func TestComputeJudgeMetrics_GivenEmptyRetrieved_ThenFilteringRateZero(t *testing.T) {
	// Given: nothing was retrieved
	judged := []string{}
	retrieved := []string{}
	relevant := []string{"a"}

	// When: we compute metrics
	_, _, _, filteringRate := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: filtering rate = 0 (no retrieved items to filter)
	assertFloat(t, "filteringRate", filteringRate, 0.0)
}

func TestComputeJudgeMetrics_GivenF1Calculation_ThenHarmonicMeanCorrect(t *testing.T) {
	// Given: precision=0.8, recall=0.5 (approximately)
	judged := []string{"a", "b", "c", "d", "x"} // 4 relevant, 1 irrelevant
	retrieved := []string{"a", "b", "c", "d", "e", "f", "g", "h", "x", "y"}
	relevant := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

	// When: we compute metrics
	precision, recall, f1, _ := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: F1 = 2 * precision * recall / (precision + recall)
	expectedF1 := 2 * precision * recall / (precision + recall)
	assertFloat(t, "f1", f1, expectedF1)
}

// --- computeFalseFilteringRate ---

func TestComputeFalseFilteringRate_GivenNoFalseDrops_ThenZero(t *testing.T) {
	// Given: judge kept all relevant retrieved items
	judged := []string{"a", "b", "c"}
	retrieved := []string{"a", "b", "c", "d"}
	relevant := []string{"a", "b"}

	// When: we compute false filtering rate
	rate := computeFalseFilteringRate(judged, retrieved, relevant)

	// Then: 0 (no relevant items were dropped)
	assertFloat(t, "falseFilteringRate", rate, 0.0)
}

func TestComputeFalseFilteringRate_GivenAllRelevantDropped_ThenOne(t *testing.T) {
	// Given: judge dropped all relevant retrieved items
	judged := []string{"x"}
	retrieved := []string{"a", "b", "x"}
	relevant := []string{"a", "b"}

	// When: we compute false filtering rate
	rate := computeFalseFilteringRate(judged, retrieved, relevant)

	// Then: 1.0 (both relevant items were falsely dropped)
	assertFloat(t, "falseFilteringRate", rate, 1.0)
}

func TestComputeFalseFilteringRate_GivenPartialDrop_ThenFraction(t *testing.T) {
	// Given: judge kept 1 of 2 relevant items
	judged := []string{"a", "x"}
	retrieved := []string{"a", "b", "x"}
	relevant := []string{"a", "b"}

	// When: we compute false filtering rate
	rate := computeFalseFilteringRate(judged, retrieved, relevant)

	// Then: 0.5 (1 of 2 relevant-retrieved was dropped)
	assertFloat(t, "falseFilteringRate", rate, 0.5)
}

func TestComputeFalseFilteringRate_GivenNoRelevantRetrieved_ThenZero(t *testing.T) {
	// Given: no relevant items were in the retrieved set
	judged := []string{"x"}
	retrieved := []string{"x", "y"}
	relevant := []string{"a", "b"}

	// When: we compute false filtering rate
	rate := computeFalseFilteringRate(judged, retrieved, relevant)

	// Then: 0 (nothing relevant to drop)
	assertFloat(t, "falseFilteringRate", rate, 0.0)
}

// --- aggregateJudgeMetrics ---

func TestAggregateJudgeMetrics_GivenEmptyInput_ThenEmptySummary(t *testing.T) {
	// Given: no per-query metrics
	var perQuery []JudgeMetrics

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: all averages are 0, empty categories
	if summary.AvgJudgePrecision != 0 {
		t.Errorf("avg precision = %v, want 0", summary.AvgJudgePrecision)
	}
	if len(summary.ByCategory) != 0 {
		t.Errorf("categories = %d, want 0", len(summary.ByCategory))
	}
	if len(summary.PerQuery) != 0 {
		t.Errorf("per_query = %d, want 0", len(summary.PerQuery))
	}
}

func TestAggregateJudgeMetrics_GivenSingleQuery_ThenAveragesEqualValues(t *testing.T) {
	// Given: a single per-query metric
	perQuery := []JudgeMetrics{
		{
			QueryID: "q1", Category: "code",
			JudgePrecision: 0.9, JudgeRecall: 0.8, JudgeF1: 0.85,
			FilteringRate: 0.3, FalseFilteringRate: 0.1, Improvement: 0.2,
		},
	}

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: averages equal the single metric's values
	assertFloat(t, "avg_precision", summary.AvgJudgePrecision, 0.9)
	assertFloat(t, "avg_recall", summary.AvgJudgeRecall, 0.8)
	assertFloat(t, "avg_f1", summary.AvgJudgeF1, 0.85)
	assertFloat(t, "avg_filtering", summary.AvgFilteringRate, 0.3)
	assertFloat(t, "avg_false_filtering", summary.AvgFalseFilteringRate, 0.1)
	assertFloat(t, "avg_improvement", summary.AvgImprovement, 0.2)
}

func TestAggregateJudgeMetrics_GivenMultipleQueries_ThenAveragesCorrect(t *testing.T) {
	// Given: 3 queries with known precision values
	perQuery := []JudgeMetrics{
		{QueryID: "q1", Category: "code", JudgePrecision: 0.6, JudgeRecall: 0.4},
		{QueryID: "q2", Category: "code", JudgePrecision: 0.8, JudgeRecall: 0.6},
		{QueryID: "q3", Category: "code", JudgePrecision: 1.0, JudgeRecall: 0.8},
	}

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: avg precision = (0.6 + 0.8 + 1.0) / 3 = 0.8
	assertFloat(t, "avg_precision", summary.AvgJudgePrecision, 0.8)
	// avg recall = (0.4 + 0.6 + 0.8) / 3 = 0.6
	assertFloat(t, "avg_recall", summary.AvgJudgeRecall, 0.6)
}

func TestAggregateJudgeMetrics_GivenMultipleCategories_ThenGroupedCorrectly(t *testing.T) {
	// Given: queries across two categories
	perQuery := []JudgeMetrics{
		{QueryID: "q1", Category: "code", JudgePrecision: 0.8, JudgeRecall: 0.7},
		{QueryID: "q2", Category: "code", JudgePrecision: 1.0, JudgeRecall: 0.9},
		{QueryID: "q3", Category: "pattern", JudgePrecision: 0.6, JudgeRecall: 0.5},
	}

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: 2 categories with correct per-category counts and averages
	if len(summary.ByCategory) != 2 {
		t.Fatalf("categories = %d, want 2", len(summary.ByCategory))
	}

	codeCat := summary.ByCategory["code"]
	if codeCat == nil {
		t.Fatal("missing 'code' category")
	}
	if codeCat.Count != 2 {
		t.Errorf("code count = %d, want 2", codeCat.Count)
	}
	// code avg precision = (0.8 + 1.0) / 2 = 0.9
	assertFloat(t, "code avg_precision", codeCat.AvgJudgePrecision, 0.9)

	patCat := summary.ByCategory["pattern"]
	if patCat == nil {
		t.Fatal("missing 'pattern' category")
	}
	if patCat.Count != 1 {
		t.Errorf("pattern count = %d, want 1", patCat.Count)
	}
	assertFloat(t, "pattern avg_precision", patCat.AvgJudgePrecision, 0.6)
}

func TestAggregateJudgeMetrics_GivenFalseFilteringRate_ThenAveragedCorrectly(t *testing.T) {
	// Given: queries with varying false filtering rates
	perQuery := []JudgeMetrics{
		{QueryID: "q1", Category: "all", FalseFilteringRate: 0.0},
		{QueryID: "q2", Category: "all", FalseFilteringRate: 0.5},
		{QueryID: "q3", Category: "all", FalseFilteringRate: 1.0},
	}

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: avg false filtering rate = 0.5
	assertFloat(t, "avg_false_filtering", summary.AvgFalseFilteringRate, 0.5)
}

func TestAggregateJudgeMetrics_GivenPerQueryPreserved_ThenAllQueriesInSummary(t *testing.T) {
	// Given: 5 per-query metrics
	perQuery := make([]JudgeMetrics, 5)
	for i := range perQuery {
		perQuery[i] = JudgeMetrics{QueryID: "q" + string(rune('1'+i)), Category: "test"}
	}

	// When: we aggregate
	summary := aggregateJudgeMetrics(perQuery)

	// Then: all 5 queries preserved in summary
	if len(summary.PerQuery) != 5 {
		t.Errorf("per_query count = %d, want 5", len(summary.PerQuery))
	}
}

// --- toSet helper (implicitly tested through computeJudgeMetrics) ---

func TestComputeJudgeMetrics_GivenDuplicatesInRelevant_ThenHandledCorrectly(t *testing.T) {
	// Given: relevant IDs with duplicates (shouldn't happen but test robustness)
	judged := []string{"a", "b"}
	retrieved := []string{"a", "b", "c"}
	relevant := []string{"a", "a", "b"} // "a" appears twice

	// When: we compute metrics
	precision, _, _, _ := computeJudgeMetrics(judged, retrieved, relevant)

	// Then: precision should still be 1.0 (both judged items are relevant)
	assertFloat(t, "precision", precision, 1.0)
}

// =============================================================================
// Helpers
// =============================================================================

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}
