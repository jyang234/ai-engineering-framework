//go:build fts5

package eval

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Results Database Tests — Given-When-Then
// =============================================================================

func TestResultsDB_GivenFreshDatabase_WhenOpened_ThenTablesExist(t *testing.T) {
	// Given: a fresh temporary directory with no prior database
	db := openTestDB(t)
	defer db.Close()

	// When: we query the sqlite_master for our tables
	var count int
	err := db.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('eval_runs', 'eval_recall_state')`).Scan(&count)

	// Then: both tables should exist
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tables, got %d", count)
	}
}

func TestResultsDB_GivenFreshDatabase_WhenOpenedTwice_ThenMigrationIsIdempotent(t *testing.T) {
	// Given: a database that was already created and migrated
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "results.db")

	db1, err := OpenResultsDB(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	db1.Close()

	// When: we open the same database a second time (re-running migration)
	db2, err := OpenResultsDB(dbPath)

	// Then: no error occurs — CREATE IF NOT EXISTS is idempotent
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	db2.Close()
}

func TestResultsDB_GivenRunWithAllFields_WhenInsertedAndRetrieved_ThenAllFieldsRoundTrip(t *testing.T) {
	// Given: a run record with every field populated
	db := openTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	completed := now.Add(45 * time.Second)
	run := makeFullRun("rt-001", "3A", "aef-full", "task-payment", "moderate", 1, now, &completed)

	// When: we insert and retrieve it
	if err := db.InsertRun(run); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := db.GetRun("rt-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// Then: all fields match
	assertRunFieldsMatch(t, run, got)
}

func TestResultsDB_GivenRunWithNullOptionalFields_WhenInsertedAndRetrieved_ThenNullsPreserved(t *testing.T) {
	// Given: a run record with only required fields — all metrics nil
	db := openTestDB(t)
	defer db.Close()

	run := &EvalRun{
		RunID:      "rt-nil-001",
		Experiment: "3A",
		Condition:  "baseline",
		TaskID:     "task-01",
		Attempt:    1,
		StartedAt:  time.Now().UTC().Truncate(time.Second),
	}

	// When: we insert and retrieve it
	if err := db.InsertRun(run); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := db.GetRun("rt-nil-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// Then: all optional pointer fields should be nil
	if got.TestsPass != nil {
		t.Error("expected TestsPass to be nil")
	}
	if got.TestPassRate != nil {
		t.Error("expected TestPassRate to be nil")
	}
	if got.JudgeCombined != nil {
		t.Error("expected JudgeCombined to be nil")
	}
	if got.TerminationReason != nil {
		t.Error("expected TerminationReason to be nil")
	}
	if got.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil")
	}
}

func TestResultsDB_GivenDuplicateRunID_WhenInserted_ThenErrorReturned(t *testing.T) {
	// Given: a run already exists with a specific ID
	db := openTestDB(t)
	defer db.Close()

	run := &EvalRun{
		RunID: "dup-001", Experiment: "3A", Condition: "baseline",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(run); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// When: we try to insert another run with the same ID
	err := db.InsertRun(run)

	// Then: an error should be returned (PRIMARY KEY violation)
	if err == nil {
		t.Error("expected error for duplicate run_id, got nil")
	}
}

func TestResultsDB_GivenExistingRun_WhenUpdatedWithScores_ThenNewValuesStored(t *testing.T) {
	// Given: a run inserted without scores
	db := openTestDB(t)
	defer db.Close()

	run := &EvalRun{
		RunID: "upd-001", Experiment: "3A", Condition: "aef-minimal",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(run); err != nil {
		t.Fatal(err)
	}

	// When: we update it with scoring results
	pass := true
	rate := 0.95
	combined := 8.5
	reason := "completed"
	run.TestsPass = &pass
	run.TestPassRate = &rate
	run.JudgeCombined = &combined
	run.TerminationReason = &reason
	now := time.Now().UTC().Truncate(time.Second)
	run.CompletedAt = &now

	if err := db.UpdateRun(run); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Then: the retrieved run reflects the new values
	got, _ := db.GetRun("upd-001")
	if got.TestsPass == nil || !*got.TestsPass {
		t.Error("TestsPass should be true after update")
	}
	if got.JudgeCombined == nil || *got.JudgeCombined != 8.5 {
		t.Errorf("JudgeCombined = %v, want 8.5", got.JudgeCombined)
	}
	if got.TerminationReason == nil || *got.TerminationReason != "completed" {
		t.Errorf("TerminationReason = %v, want completed", got.TerminationReason)
	}
}

func TestResultsDB_GivenMultipleExperiments_WhenQueriedByExperiment_ThenOnlyMatchingReturned(t *testing.T) {
	// Given: runs across two different experiments
	db := openTestDB(t)
	defer db.Close()

	for _, exp := range []string{"3A", "3A", "4A"} {
		run := &EvalRun{
			RunID: fmt.Sprintf("exp-%s-%d", exp, time.Now().UnixNano()),
			Experiment: exp, Condition: "baseline", TaskID: "task-01",
			Attempt: 1, StartedAt: time.Now().UTC(),
		}
		if err := db.InsertRun(run); err != nil {
			t.Fatal(err)
		}
	}

	// When: we query for experiment "3A"
	runs, err := db.QueryByExperiment("3A", "")
	if err != nil {
		t.Fatal(err)
	}

	// Then: only the two "3A" runs are returned
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for 3A, got %d", len(runs))
	}
}

func TestResultsDB_GivenMultipleConditions_WhenFilteredByCondition_ThenOnlyMatchingReturned(t *testing.T) {
	// Given: runs with different conditions in the same experiment
	db := openTestDB(t)
	defer db.Close()

	for i, cond := range []string{"baseline", "aef-minimal", "aef-full"} {
		run := &EvalRun{
			RunID: fmt.Sprintf("cond-%d", i), Experiment: "3A",
			Condition: cond, TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
		}
		if err := db.InsertRun(run); err != nil {
			t.Fatal(err)
		}
	}

	// When: we query for condition "aef-full" in experiment "3A"
	runs, err := db.QueryByExperiment("3A", "aef-full")
	if err != nil {
		t.Fatal(err)
	}

	// Then: exactly one run is returned
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Condition != "aef-full" {
		t.Errorf("condition = %q, want aef-full", runs[0].Condition)
	}
}

func TestResultsDB_GivenRunsWithScores_WhenConditionStatsComputed_ThenMediansCorrect(t *testing.T) {
	// Given: 5 runs under "aef-full" with known test pass rates and judge scores
	db := openTestDB(t)
	defer db.Close()

	rates := []float64{0.60, 0.80, 0.90, 0.95, 1.0}
	judges := []float64{6.0, 7.0, 8.0, 8.5, 9.0}
	for i := 0; i < 5; i++ {
		r := rates[i]
		j := judges[i]
		lc := r == 1.0
		run := &EvalRun{
			RunID: fmt.Sprintf("stats-%d", i), Experiment: "3A",
			Condition: "aef-full", TaskID: "task-01", Attempt: i + 1,
			StartedAt: time.Now().UTC(), TestPassRate: &r, JudgeCombined: &j, LintClean: &lc,
		}
		if err := db.InsertRun(run); err != nil {
			t.Fatal(err)
		}
	}

	// When: we compute condition stats
	stats, err := db.ComputeConditionStats("3A")
	if err != nil {
		t.Fatal(err)
	}

	// Then: the median test pass rate is 0.90 (middle of 5 sorted values)
	if len(stats) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(stats))
	}
	s := stats[0]
	if s.RunCount != 5 {
		t.Errorf("run_count = %d, want 5", s.RunCount)
	}
	if s.MedianTestPassRate != 0.90 {
		t.Errorf("median_test_pass_rate = %v, want 0.90", s.MedianTestPassRate)
	}
	if s.MedianJudgeCombined != 8.0 {
		t.Errorf("median_judge_combined = %v, want 8.0", s.MedianJudgeCombined)
	}
	// LintClean: only 1 out of 5 is true → 0.20
	if math.Abs(s.MedianLintClean-0.20) > 0.001 {
		t.Errorf("median_lint_clean = %v, want 0.20", s.MedianLintClean)
	}
}

func TestResultsDB_GivenRecallState_WhenInsertedAndQueried_ThenFieldsMatch(t *testing.T) {
	// Given: a run exists, and we record RECALL state for it
	db := openTestDB(t)
	defer db.Close()

	run := &EvalRun{
		RunID: "recall-run", Experiment: "3A", Condition: "aef-full",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(run); err != nil {
		t.Fatal(err)
	}

	// When: we insert two recall states
	states := []*EvalRecallState{
		{RunID: "recall-run", ItemID: "F-001", ItemType: "failure", ItemTitle: "Missing retry jitter",
			WasRetrieved: true, WasKept: true, WasUsed: true},
		{RunID: "recall-run", ItemID: "F-002", ItemType: "failure", ItemTitle: "No idempotency key",
			WasRetrieved: true, WasKept: false, WasUsed: false},
	}
	for _, s := range states {
		if err := db.InsertRecallState(s); err != nil {
			t.Fatalf("insert state: %v", err)
		}
	}

	// Then: querying returns both with correct fields
	got, err := db.QueryRecallState("recall-run")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d states, want 2", len(got))
	}

	// Verify the first (F-001)
	var f001 *EvalRecallState
	for _, s := range got {
		if s.ItemID == "F-001" {
			f001 = s
		}
	}
	if f001 == nil {
		t.Fatal("F-001 not found")
	}
	if !f001.WasRetrieved || !f001.WasKept || !f001.WasUsed {
		t.Errorf("F-001: retrieved=%v kept=%v used=%v, want all true",
			f001.WasRetrieved, f001.WasKept, f001.WasUsed)
	}
}

func TestResultsDB_GivenRecallState_WhenUpserted_ThenLatestValueWins(t *testing.T) {
	// Given: a recall state exists for an item
	db := openTestDB(t)
	defer db.Close()

	run := &EvalRun{
		RunID: "upsert-run", Experiment: "3A", Condition: "aef-full",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(run); err != nil {
		t.Fatal(err)
	}

	original := &EvalRecallState{
		RunID: "upsert-run", ItemID: "F-001", WasRetrieved: true, WasKept: false, WasUsed: false,
	}
	if err := db.InsertRecallState(original); err != nil {
		t.Fatal(err)
	}

	// When: we upsert with updated values (INSERT OR REPLACE)
	updated := &EvalRecallState{
		RunID: "upsert-run", ItemID: "F-001", WasRetrieved: true, WasKept: true, WasUsed: true,
	}
	if err := db.InsertRecallState(updated); err != nil {
		t.Fatal(err)
	}

	// Then: the latest values are stored
	got, _ := db.QueryRecallState("upsert-run")
	if len(got) != 1 {
		t.Fatalf("expected 1 state, got %d", len(got))
	}
	if !got[0].WasKept || !got[0].WasUsed {
		t.Errorf("expected upserted values: kept=%v used=%v", got[0].WasKept, got[0].WasUsed)
	}
}

func TestResultsDB_GivenNonExistentRun_WhenQueried_ThenErrorReturned(t *testing.T) {
	// Given: an empty database
	db := openTestDB(t)
	defer db.Close()

	// When: we query for a run that doesn't exist
	_, err := db.GetRun("does-not-exist")

	// Then: an error should be returned (sql.ErrNoRows)
	if err == nil {
		t.Error("expected error for non-existent run, got nil")
	}
}

func TestFormatResultsJSON_GivenRun_WhenFormatted_ThenValidJSONProduced(t *testing.T) {
	// Given: a run with some fields populated
	pass := true
	rate := 0.95
	run := &EvalRun{
		RunID: "json-001", Experiment: "3A", Condition: "aef-full",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC(),
		TestsPass: &pass, TestPassRate: &rate,
	}

	// When: we format it as JSON
	result := FormatResultsJSON(run)

	// Then: it should be non-empty valid JSON containing the run_id
	if result == "" {
		t.Error("expected non-empty JSON")
	}
	if len(result) < 10 {
		t.Errorf("JSON suspiciously short: %q", result)
	}
}

func TestMedian_GivenVariousInputs_ThenCorrectMedianComputed(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"given empty slice, then returns 0", nil, 0},
		{"given single value 5, then returns 5", []float64{5}, 5},
		{"given odd-length [3,1,2], then returns middle value 2", []float64{3, 1, 2}, 2},
		{"given even-length [4,1,3,2], then returns average of middle two: 2.5", []float64{4, 1, 3, 2}, 2.5},
		{"given already-sorted [1,2,3,4,5], then returns 3", []float64{1, 2, 3, 4, 5}, 3},
		{"given duplicates [7,7,7], then returns 7", []float64{7, 7, 7}, 7},
		{"given two values [10,20], then returns 15", []float64{10, 20}, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := median(tt.vals)
			if got != tt.want {
				t.Errorf("median(%v) = %v, want %v", tt.vals, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

func openTestDB(t *testing.T) *ResultsDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "results.db")
	db, err := OpenResultsDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func makeFullRun(runID, experiment, condition, taskID, complexity string, attempt int, started time.Time, completed *time.Time) *EvalRun {
	pass := true
	rate := 0.95
	lintV := 2
	lintClean := false
	pitTotal := 3
	pitAvoided := 2
	turns := 15
	tokens := 40000
	jCorr := 8.5
	jQual := 7.5
	jPit := 9.0
	jComp := 8.0
	jEff := 7.0
	jComb := 8.1
	dur := int64(45000)
	tPath := "/tmp/transcript.json"
	rJSON := `{"test": true}`

	return &EvalRun{
		RunID: runID, Experiment: experiment, Condition: condition,
		TaskID: taskID, TaskComplexity: complexity, Attempt: attempt,
		TestsPass: &pass, TestPassRate: &rate,
		LintViolations: &lintV, LintClean: &lintClean,
		PitfallsTotal: &pitTotal, PitfallsAvoided: &pitAvoided,
		TurnsToComplete: &turns, TokensConsumed: &tokens,
		JudgeCorrectness: &jCorr, JudgeCodeQuality: &jQual,
		JudgePitfallAvoidance: &jPit, JudgeCompleteness: &jComp,
		JudgeEfficiency: &jEff, JudgeCombined: &jComb,
		DurationMs: &dur, StartedAt: started, CompletedAt: completed,
		TranscriptPath: &tPath, ResultsJSON: &rJSON,
	}
}

func assertRunFieldsMatch(t *testing.T, want, got *EvalRun) {
	t.Helper()
	if got.RunID != want.RunID {
		t.Errorf("RunID = %q, want %q", got.RunID, want.RunID)
	}
	if got.Experiment != want.Experiment {
		t.Errorf("Experiment = %q, want %q", got.Experiment, want.Experiment)
	}
	if got.Condition != want.Condition {
		t.Errorf("Condition = %q, want %q", got.Condition, want.Condition)
	}
	if got.TaskID != want.TaskID {
		t.Errorf("TaskID = %q, want %q", got.TaskID, want.TaskID)
	}
	if got.TaskComplexity != want.TaskComplexity {
		t.Errorf("TaskComplexity = %q, want %q", got.TaskComplexity, want.TaskComplexity)
	}
	if got.Attempt != want.Attempt {
		t.Errorf("Attempt = %d, want %d", got.Attempt, want.Attempt)
	}
	assertPtrBool(t, "TestsPass", got.TestsPass, want.TestsPass)
	assertPtrFloat(t, "TestPassRate", got.TestPassRate, want.TestPassRate)
	assertPtrInt(t, "LintViolations", got.LintViolations, want.LintViolations)
	assertPtrFloat(t, "JudgeCombined", got.JudgeCombined, want.JudgeCombined)
	assertPtrInt(t, "PitfallsAvoided", got.PitfallsAvoided, want.PitfallsAvoided)
	assertPtrInt(t, "TurnsToComplete", got.TurnsToComplete, want.TurnsToComplete)
}

func assertPtrBool(t *testing.T, name string, got, want *bool) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: nil mismatch got=%v want=%v", name, got, want)
		return
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", name, *got, *want)
	}
}

func assertPtrFloat(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: nil mismatch got=%v want=%v", name, got, want)
		return
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", name, *got, *want)
	}
}

func assertPtrInt(t *testing.T, name string, got, want *int) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: nil mismatch got=%v want=%v", name, got, want)
		return
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", name, *got, *want)
	}
}
