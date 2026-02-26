package eval

import (
	"testing"
)

// =============================================================================
// Plan Review & Audit Trail Tests — Given-When-Then
// =============================================================================

// --- PlanReviewSummary computation ---

func TestPlanReviewSummary_GivenAllDangerousDetected_ThenDetectionRateOne(t *testing.T) {
	// Given: 3 dangerous scenarios, all detected
	results := []PlanReviewResult{
		{ScenarioID: "s1", IsDangerous: true, FailureDetected: true},
		{ScenarioID: "s2", IsDangerous: true, FailureDetected: true},
		{ScenarioID: "s3", IsDangerous: true, FailureDetected: true},
	}

	// When: we compute summary stats
	summary := computePlanReviewSummary(results)

	// Then: detection rate = 1.0, no false alarms
	if summary.DetectionRate != 1.0 {
		t.Errorf("detection_rate = %v, want 1.0", summary.DetectionRate)
	}
	if summary.DangerousCount != 3 {
		t.Errorf("dangerous_count = %d, want 3", summary.DangerousCount)
	}
	if summary.FalseAlarmRate != 0 {
		t.Errorf("false_alarm_rate = %v, want 0", summary.FalseAlarmRate)
	}
}

func TestPlanReviewSummary_GivenNoDangerousDetected_ThenDetectionRateZero(t *testing.T) {
	// Given: 2 dangerous scenarios, none detected
	results := []PlanReviewResult{
		{ScenarioID: "s1", IsDangerous: true, FailureDetected: false},
		{ScenarioID: "s2", IsDangerous: true, FailureDetected: false},
	}

	// When: we compute summary stats
	summary := computePlanReviewSummary(results)

	// Then: detection rate = 0
	if summary.DetectionRate != 0 {
		t.Errorf("detection_rate = %v, want 0", summary.DetectionRate)
	}
}

func TestPlanReviewSummary_GivenMixedDangerousAndSafe_ThenRatesCorrect(t *testing.T) {
	// Given: 2 dangerous (1 detected), 2 safe (1 false alarm)
	results := []PlanReviewResult{
		{ScenarioID: "d1", IsDangerous: true, FailureDetected: true},
		{ScenarioID: "d2", IsDangerous: true, FailureDetected: false},
		{ScenarioID: "s1", IsDangerous: false, FalseAlarm: false},
		{ScenarioID: "s2", IsDangerous: false, FalseAlarm: true},
	}

	// When: we compute summary stats
	summary := computePlanReviewSummary(results)

	// Then: detection rate = 0.5, false alarm rate = 0.5
	if summary.DetectionRate != 0.5 {
		t.Errorf("detection_rate = %v, want 0.5", summary.DetectionRate)
	}
	if summary.FalseAlarmRate != 0.5 {
		t.Errorf("false_alarm_rate = %v, want 0.5", summary.FalseAlarmRate)
	}
	if summary.DangerousCount != 2 {
		t.Errorf("dangerous_count = %d, want 2", summary.DangerousCount)
	}
	if summary.SafeCount != 2 {
		t.Errorf("safe_count = %d, want 2", summary.SafeCount)
	}
}

func TestPlanReviewSummary_GivenOnlySafeScenarios_ThenDetectionRateZeroDivisionSafe(t *testing.T) {
	// Given: only safe scenarios (no dangerous ones)
	results := []PlanReviewResult{
		{ScenarioID: "s1", IsDangerous: false, FalseAlarm: false},
		{ScenarioID: "s2", IsDangerous: false, FalseAlarm: false},
	}

	// When: we compute summary stats
	summary := computePlanReviewSummary(results)

	// Then: detection rate = 0 (no dangerous to detect), false alarm rate = 0
	if summary.DetectionRate != 0 {
		t.Errorf("detection_rate = %v, want 0 (no dangerous scenarios)", summary.DetectionRate)
	}
	if summary.FalseAlarmRate != 0 {
		t.Errorf("false_alarm_rate = %v, want 0", summary.FalseAlarmRate)
	}
}

func TestPlanReviewSummary_GivenEmptyResults_ThenAllZeros(t *testing.T) {
	// Given: no results
	results := []PlanReviewResult{}

	// When: we compute summary stats
	summary := computePlanReviewSummary(results)

	// Then: all counts and rates are zero
	if summary.TotalScenarios != 0 {
		t.Errorf("total = %d, want 0", summary.TotalScenarios)
	}
	if summary.DetectionRate != 0 || summary.FalseAlarmRate != 0 {
		t.Error("expected zero rates for empty results")
	}
}

// --- PlanScenario types ---

func TestPlanScenario_GivenDangerousScenario_ThenFieldsPopulated(t *testing.T) {
	// Given: a plan scenario marked as dangerous
	scenario := PlanScenario{
		ID:          "scenario-001",
		Plan:        "Use fixed retry interval without jitter",
		IsDangerous: true,
		FailureID:   "F-jitter",
		FailureDesc: "Missing jitter causes thundering herd",
	}

	// Then: all fields are accessible and correct
	if !scenario.IsDangerous {
		t.Error("expected IsDangerous=true")
	}
	if scenario.FailureID != "F-jitter" {
		t.Errorf("FailureID = %q, want F-jitter", scenario.FailureID)
	}
}

func TestPlanScenario_GivenSafeScenario_ThenNoFailureID(t *testing.T) {
	// Given: a safe plan scenario
	scenario := PlanScenario{
		ID:          "scenario-002",
		Plan:        "Use exponential backoff with random jitter",
		IsDangerous: false,
	}

	// Then: no failure reference
	if scenario.IsDangerous {
		t.Error("expected IsDangerous=false")
	}
	if scenario.FailureID != "" {
		t.Errorf("safe scenario should have empty FailureID, got %q", scenario.FailureID)
	}
}

// --- truncateForPrompt ---

func TestTruncateForPrompt_GivenShortString_ThenUnchanged(t *testing.T) {
	// Given: a string shorter than the max
	input := "hello world"

	// When: we truncate with a high limit
	result := truncateForPrompt(input, 100)

	// Then: the string is unchanged
	if result != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}
}

func TestTruncateForPrompt_GivenLongString_ThenTruncatedWithEllipsis(t *testing.T) {
	// Given: a string longer than the limit
	input := "abcdefghijklmnopqrstuvwxyz"

	// When: we truncate to 10 characters
	result := truncateForPrompt(input, 10)

	// Then: it's truncated to 10 chars + "..."
	if result != "abcdefghij..." {
		t.Errorf("got %q, want %q", result, "abcdefghij...")
	}
	if len(result) != 13 { // 10 + 3 for "..."
		t.Errorf("length = %d, want 13", len(result))
	}
}

func TestTruncateForPrompt_GivenExactLength_ThenUnchanged(t *testing.T) {
	// Given: a string exactly at the limit
	input := "12345"

	// When: we truncate with limit=5
	result := truncateForPrompt(input, 5)

	// Then: unchanged
	if result != "12345" {
		t.Errorf("got %q, want %q", result, "12345")
	}
}

// --- AuditTrailCoverage ---

func TestComputeAuditTrailCoverage_GivenPerfectCoverage_ThenAllRatesOne(t *testing.T) {
	// Given: 10 search calls, all with queries and judgments; 5 changes all with decisions
	cov := ComputeAuditTrailCoverage(10, 10, 5, 10, 5)

	// Then: all coverage rates = 1.0
	if cov.QueryCoverage != 1.0 {
		t.Errorf("query_coverage = %v, want 1.0", cov.QueryCoverage)
	}
	if cov.JudgmentCoverage != 1.0 {
		t.Errorf("judgment_coverage = %v, want 1.0", cov.JudgmentCoverage)
	}
	if cov.DecisionCoverage != 1.0 {
		t.Errorf("decision_coverage = %v, want 1.0", cov.DecisionCoverage)
	}
}

func TestComputeAuditTrailCoverage_GivenZeroSearchCalls_ThenZeroCoverage(t *testing.T) {
	// Given: no search calls were made
	cov := ComputeAuditTrailCoverage(0, 0, 3, 0, 5)

	// Then: query and judgment coverage = 0 (no search calls to cover)
	if cov.QueryCoverage != 0 {
		t.Errorf("query_coverage = %v, want 0", cov.QueryCoverage)
	}
	if cov.JudgmentCoverage != 0 {
		t.Errorf("judgment_coverage = %v, want 0", cov.JudgmentCoverage)
	}
	// Decision coverage: 3/5 = 0.6
	if cov.DecisionCoverage != 0.6 {
		t.Errorf("decision_coverage = %v, want 0.6", cov.DecisionCoverage)
	}
}

func TestComputeAuditTrailCoverage_GivenPartialCoverage_ThenFractionalRates(t *testing.T) {
	// Given: 10 search calls, 7 with queries, 5 with judgments; 4 changes, 2 with decisions
	cov := ComputeAuditTrailCoverage(7, 5, 2, 10, 4)

	// Then: query=0.7, judgment=0.5, decision=0.5
	if cov.QueryCoverage != 0.7 {
		t.Errorf("query_coverage = %v, want 0.7", cov.QueryCoverage)
	}
	if cov.JudgmentCoverage != 0.5 {
		t.Errorf("judgment_coverage = %v, want 0.5", cov.JudgmentCoverage)
	}
	if cov.DecisionCoverage != 0.5 {
		t.Errorf("decision_coverage = %v, want 0.5", cov.DecisionCoverage)
	}
}

func TestComputeAuditTrailCoverage_GivenEntriesExceedCalls_ThenCappedAtOne(t *testing.T) {
	// Given: more query entries than search calls (edge case)
	cov := ComputeAuditTrailCoverage(15, 12, 8, 10, 5)

	// Then: all rates capped at 1.0
	if cov.QueryCoverage != 1.0 {
		t.Errorf("query_coverage = %v, want 1.0 (capped)", cov.QueryCoverage)
	}
	if cov.JudgmentCoverage != 1.0 {
		t.Errorf("judgment_coverage = %v, want 1.0 (capped)", cov.JudgmentCoverage)
	}
	if cov.DecisionCoverage != 1.0 {
		t.Errorf("decision_coverage = %v, want 1.0 (capped)", cov.DecisionCoverage)
	}
}

func TestComputeAuditTrailCoverage_GivenZeroChanges_ThenDecisionCoverageZero(t *testing.T) {
	// Given: no significant changes were made
	cov := ComputeAuditTrailCoverage(5, 5, 0, 10, 0)

	// Then: decision coverage = 0 (division by zero guarded)
	if cov.DecisionCoverage != 0 {
		t.Errorf("decision_coverage = %v, want 0", cov.DecisionCoverage)
	}
	// Raw counts preserved
	if cov.TotalSearchCalls != 10 {
		t.Errorf("total_search_calls = %d, want 10", cov.TotalSearchCalls)
	}
	if cov.QueryEntries != 5 {
		t.Errorf("query_entries = %d, want 5", cov.QueryEntries)
	}
}

func TestComputeAuditTrailCoverage_GivenRawCountsPreserved_ThenFieldsMatch(t *testing.T) {
	// Given: specific entry counts
	cov := ComputeAuditTrailCoverage(3, 7, 2, 20, 10)

	// Then: raw counts are stored verbatim
	if cov.QueryEntries != 3 {
		t.Errorf("query_entries = %d, want 3", cov.QueryEntries)
	}
	if cov.JudgmentEntries != 7 {
		t.Errorf("judgment_entries = %d, want 7", cov.JudgmentEntries)
	}
	if cov.DecisionEntries != 2 {
		t.Errorf("decision_entries = %d, want 2", cov.DecisionEntries)
	}
	if cov.TotalSearchCalls != 20 {
		t.Errorf("total_search_calls = %d, want 20", cov.TotalSearchCalls)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// computePlanReviewSummary is a test helper that replicates the summary computation
// from RunPlanReview to allow testing the logic in isolation.
func computePlanReviewSummary(results []PlanReviewResult) *PlanReviewSummary {
	summary := &PlanReviewSummary{
		TotalScenarios: len(results),
		Results:        results,
	}

	var detected, falseAlarms int
	for _, r := range results {
		if r.IsDangerous {
			summary.DangerousCount++
			if r.FailureDetected {
				detected++
			}
		} else {
			summary.SafeCount++
			if r.FalseAlarm {
				falseAlarms++
			}
		}
	}

	if summary.DangerousCount > 0 {
		summary.DetectionRate = float64(detected) / float64(summary.DangerousCount)
	}
	if summary.SafeCount > 0 {
		summary.FalseAlarmRate = float64(falseAlarms) / float64(summary.SafeCount)
	}

	return summary
}
