//go:build fts5

package eval

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Level 3-4 Report Tests — Given-When-Then
// =============================================================================

// --- EvalFrameworkClaims ---

func TestEvalFrameworkClaims_GivenDefaults_ThenSevenClaimsReturned(t *testing.T) {
	// Given/When: we request the claims
	claims := EvalFrameworkClaims()

	// Then: exactly 7 claims matching the spec's success criteria table
	if len(claims) != 7 {
		t.Fatalf("got %d claims, want 7", len(claims))
	}
}

func TestEvalFrameworkClaims_GivenAllClaims_ThenEachHasRequiredFields(t *testing.T) {
	// Given: all claims
	claims := EvalFrameworkClaims()

	// Then: each claim has ID, Description, Experiment, and Metric
	for _, c := range claims {
		if c.ID == "" {
			t.Error("claim missing ID")
		}
		if c.Description == "" {
			t.Errorf("claim %s missing Description", c.ID)
		}
		if c.Experiment == "" {
			t.Errorf("claim %s missing Experiment", c.ID)
		}
		if c.Metric == "" {
			t.Errorf("claim %s missing Metric", c.ID)
		}
	}
}

func TestEvalFrameworkClaims_GivenC1_ThenMatchesSpecDefectRate(t *testing.T) {
	// Given: the first claim
	claims := EvalFrameworkClaims()
	c1 := claims[0]

	// Then: matches spec "AEF reduces defects" with >=10 point threshold
	if c1.ID != "C1" {
		t.Errorf("first claim ID = %q, want C1", c1.ID)
	}
	if c1.Experiment != "3A" {
		t.Errorf("C1 experiment = %q, want 3A", c1.Experiment)
	}
	if c1.Threshold != 10.0 {
		t.Errorf("C1 threshold = %f, want 10.0", c1.Threshold)
	}
	if c1.Direction != HigherIsBetter {
		t.Error("C1 direction should be HigherIsBetter")
	}
}

func TestEvalFrameworkClaims_GivenC3_ThenMatchesRepeatFailureSpec(t *testing.T) {
	// Given: claim C3
	claims := EvalFrameworkClaims()
	var c3 Claim
	for _, c := range claims {
		if c.ID == "C3" {
			c3 = c
		}
	}

	// Then: matches spec ">=25 point prevention delta"
	if c3.Experiment != "3B" {
		t.Errorf("C3 experiment = %q, want 3B", c3.Experiment)
	}
	if c3.Threshold != 25.0 {
		t.Errorf("C3 threshold = %f, want 25.0", c3.Threshold)
	}
}

func TestEvalFrameworkClaims_GivenC7_ThenAbsoluteThreshold(t *testing.T) {
	// Given: claim C7 (plan-review, no comparison condition)
	claims := EvalFrameworkClaims()
	var c7 Claim
	for _, c := range claims {
		if c.ID == "C7" {
			c7 = c
		}
	}

	// Then: condition2 is empty (absolute threshold), threshold = 60
	if c7.Condition2 != "" {
		t.Errorf("C7 condition2 = %q, want empty (absolute threshold)", c7.Condition2)
	}
	if c7.Threshold != 60.0 {
		t.Errorf("C7 threshold = %f, want 60.0", c7.Threshold)
	}
}

// --- experimentDescription ---

func TestExperimentDescription_GivenKnownExperiment_ThenDescriptionReturned(t *testing.T) {
	// Given: a known experiment ID
	tests := map[string]string{
		"3A": "Defect Rate Comparison",
		"3B": "Repeat Failure Prevention",
		"3C": "Session Knowledge Accumulation",
		"3D": "Hook Adherence Rate",
	}

	for id, expected := range tests {
		// When: we get the description
		desc := experimentDescription(id)

		// Then: it should contain the expected substring
		if !strings.Contains(desc, expected) {
			t.Errorf("description(%q) = %q, want contains %q", id, desc, expected)
		}
	}
}

func TestExperimentDescription_GivenUnknownExperiment_ThenUnknownReturned(t *testing.T) {
	// Given: an unknown experiment ID
	desc := experimentDescription("99Z")

	// Then: "Unknown" is returned
	if desc != "Unknown experiment" {
		t.Errorf("desc = %q, want 'Unknown experiment'", desc)
	}
}

// --- L3ReportGenerator ---

func TestL3ReportGenerator_GivenEmptyDB_WhenReportGenerated_ThenEmptyReport(t *testing.T) {
	// Given: an empty results database
	db := openTestDB(t)
	defer db.Close()
	gen := NewL3ReportGenerator(db)

	// When: we generate a report for experiment 3A
	report, err := gen.GenerateExperimentReport("3A")

	// Then: report has the experiment ID but no conditions or comparisons
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Experiment != "3A" {
		t.Errorf("experiment = %q, want 3A", report.Experiment)
	}
	if len(report.Conditions) != 0 {
		t.Errorf("conditions = %d, want 0 for empty db", len(report.Conditions))
	}
}

func TestL3ReportGenerator_GivenRunsWithScores_WhenReportGenerated_ThenConditionSummariesPopulated(t *testing.T) {
	// Given: runs across 2 conditions with judge scores
	db := openTestDB(t)
	defer db.Close()

	insertScoredRuns(t, db, "3A", "baseline", 5, 6.0)
	insertScoredRuns(t, db, "3A", "aef-full", 5, 8.0)

	gen := NewL3ReportGenerator(db)

	// When: we generate a report
	report, err := gen.GenerateExperimentReport("3A")

	// Then: two conditions with correct mean judge_combined
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(report.Conditions) != 2 {
		t.Fatalf("conditions = %d, want 2", len(report.Conditions))
	}

	for _, c := range report.Conditions {
		if c.Condition == "baseline" && c.MeanJudgeCombined != 6.0 {
			t.Errorf("baseline mean = %f, want 6.0", c.MeanJudgeCombined)
		}
		if c.Condition == "aef-full" && c.MeanJudgeCombined != 8.0 {
			t.Errorf("aef-full mean = %f, want 8.0", c.MeanJudgeCombined)
		}
	}
}

func TestL3ReportGenerator_GivenTwoConditions_WhenReportGenerated_ThenComparisonsIncluded(t *testing.T) {
	// Given: runs for two conditions
	db := openTestDB(t)
	defer db.Close()

	insertScoredRuns(t, db, "3A", "baseline", 5, 5.0)
	insertScoredRuns(t, db, "3A", "aef-full", 5, 8.0)

	gen := NewL3ReportGenerator(db)

	// When: we generate a report
	report, _ := gen.GenerateExperimentReport("3A")

	// Then: at least one comparison should exist
	if len(report.Comparisons) == 0 {
		t.Error("expected at least one pairwise comparison")
	}
}

func TestL3ReportGenerator_GivenClaimExperimentMatch_WhenReportGenerated_ThenClaimsValidated(t *testing.T) {
	// Given: runs that satisfy claim C1 (aef-full > baseline by >=10 on combined)
	db := openTestDB(t)
	defer db.Close()

	insertScoredRuns(t, db, "3A", "baseline", 5, 4.0)
	insertScoredRuns(t, db, "3A", "aef-full", 5, 15.0) // diff = 11 > 10

	gen := NewL3ReportGenerator(db)

	// When: we generate a report
	report, _ := gen.GenerateExperimentReport("3A")

	// Then: claim C1 should be present and validated
	var foundC1 bool
	for _, c := range report.Claims {
		if c.ClaimID == "C1" {
			foundC1 = true
			if !c.Validated {
				t.Errorf("C1 should be validated (diff=11 >= threshold=10), evidence: %s", c.Evidence)
			}
		}
	}
	if !foundC1 {
		t.Error("claim C1 should be present for experiment 3A")
	}
}

func TestL3ReportGenerator_GivenInsufficientDelta_WhenClaimValidated_ThenNotValid(t *testing.T) {
	// Given: runs where aef-full barely outperforms baseline (diff = 5 < 10)
	db := openTestDB(t)
	defer db.Close()

	insertScoredRuns(t, db, "3A", "baseline", 5, 5.0)
	insertScoredRuns(t, db, "3A", "aef-full", 5, 10.0) // diff = 5 < 10

	gen := NewL3ReportGenerator(db)

	// When: we generate a report
	report, _ := gen.GenerateExperimentReport("3A")

	// Then: claim C1 should NOT be validated
	for _, c := range report.Claims {
		if c.ClaimID == "C1" && c.Validated {
			t.Error("C1 should NOT be validated when diff < 10")
		}
	}
}

// --- FormatExperimentReport ---

func TestFormatExperimentReport_GivenReport_WhenFormatted_ThenContainsExperimentID(t *testing.T) {
	// Given: a report
	report := &ExperimentReport{
		Experiment:  "3A",
		Description: "Test experiment",
		Conditions: []ConditionSummary{
			{Condition: "baseline", RunCount: 5, MeanJudgeCombined: 6.0},
		},
	}

	// When: we format it
	output := FormatExperimentReport(report)

	// Then: output contains the experiment ID
	if !strings.Contains(output, "3A") {
		t.Error("output should contain experiment ID")
	}
	if !strings.Contains(output, "baseline") {
		t.Error("output should contain condition name")
	}
}

func TestFormatExperimentReport_GivenClaims_WhenFormatted_ThenClaimTablePresent(t *testing.T) {
	// Given: a report with claims
	report := &ExperimentReport{
		Experiment: "3A",
		Claims: []ClaimResult{
			{ClaimID: "C1", Description: "Test claim", Measured: 15.0, Validated: true},
		},
	}

	// When: we format
	output := FormatExperimentReport(report)

	// Then: claim table header and claim ID are present
	if !strings.Contains(output, "Claim Validation") {
		t.Error("output should contain claim validation header")
	}
	if !strings.Contains(output, "C1") {
		t.Error("output should contain claim ID C1")
	}
	if !strings.Contains(output, "YES") {
		t.Error("output should contain YES for validated claim")
	}
}

// --- FormatExperimentReportJSON ---

func TestFormatExperimentReportJSON_GivenReport_ThenValidJSON(t *testing.T) {
	// Given: a report
	report := &ExperimentReport{Experiment: "3A"}

	// When: we format as JSON
	out, err := FormatExperimentReportJSON(report)

	// Then: no error and contains experiment
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, `"experiment": "3A"`) {
		t.Error("JSON should contain experiment field")
	}
}

// --- FormatScorecard ---

func TestFormatScorecard_GivenReports_ThenContainsHeader(t *testing.T) {
	// Given: reports with claims
	reports := []*ExperimentReport{
		{
			Experiment: "3A",
			Claims:     []ClaimResult{{ClaimID: "C1", Validated: true, Measured: 11.0}},
		},
	}

	// When: we format the scorecard
	output := FormatScorecard(reports)

	// Then: contains header and dimension
	if !strings.Contains(output, "Measured Scorecard") {
		t.Error("scorecard should contain header")
	}
}

// =============================================================================
// Helpers
// =============================================================================

func insertScoredRuns(t *testing.T, db *ResultsDB, experiment, condition string, count int, judgeScore float64) {
	t.Helper()
	for i := 0; i < count; i++ {
		run := &EvalRun{
			RunID:      experiment + "-" + condition + "-" + time.Now().Format("150405.000") + "-" + string(rune('a'+i)),
			Experiment: experiment,
			Condition:  condition,
			TaskID:     "task-01",
			Attempt:    i + 1,
			StartedAt:  time.Now().UTC(),
			JudgeCombined: &judgeScore,
		}
		if err := db.InsertRun(run); err != nil {
			t.Fatalf("insert run: %v", err)
		}
	}
}
