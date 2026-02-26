//go:build fts5

package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResultsDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eval-results-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "results.db")
	db, err := OpenResultsDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	completed := now.Add(30 * time.Second)

	t.Run("InsertAndGet", func(t *testing.T) {
		pass := true
		passRate := 1.0
		lintV := 0
		lintClean := true
		pitTotal := 3
		pitAvoided := 2
		turns := 12
		tokens := 50000
		jCorr := 8.0
		jQual := 7.5
		jPit := 9.0
		jComp := 8.5
		jEff := 7.0
		jComb := 8.1
		dur := int64(30000)
		tPath := "/tmp/transcript.json"

		run := &EvalRun{
			RunID:                 "run-001",
			Experiment:            "3A",
			Condition:             "aef-full",
			TaskID:                "task-01",
			TaskComplexity:        "moderate",
			Attempt:               1,
			TestsPass:             &pass,
			TestPassRate:          &passRate,
			LintViolations:        &lintV,
			LintClean:             &lintClean,
			PitfallsTotal:         &pitTotal,
			PitfallsAvoided:       &pitAvoided,
			TurnsToComplete:       &turns,
			TokensConsumed:        &tokens,
			JudgeCorrectness:      &jCorr,
			JudgeCodeQuality:      &jQual,
			JudgePitfallAvoidance: &jPit,
			JudgeCompleteness:     &jComp,
			JudgeEfficiency:       &jEff,
			JudgeCombined:         &jComb,
			DurationMs:            &dur,
			StartedAt:             now,
			CompletedAt:           &completed,
			TranscriptPath:        &tPath,
		}

		if err := db.InsertRun(run); err != nil {
			t.Fatalf("insert: %v", err)
		}

		got, err := db.GetRun("run-001")
		if err != nil {
			t.Fatalf("get: %v", err)
		}

		if got.RunID != "run-001" {
			t.Errorf("run_id = %q, want %q", got.RunID, "run-001")
		}
		if got.Experiment != "3A" {
			t.Errorf("experiment = %q, want %q", got.Experiment, "3A")
		}
		if got.Condition != "aef-full" {
			t.Errorf("condition = %q, want %q", got.Condition, "aef-full")
		}
		if got.JudgeCombined == nil || *got.JudgeCombined != 8.1 {
			t.Errorf("judge_combined = %v, want 8.1", got.JudgeCombined)
		}
		if got.PitfallsAvoided == nil || *got.PitfallsAvoided != 2 {
			t.Errorf("pitfalls_avoided = %v, want 2", got.PitfallsAvoided)
		}
	})

	t.Run("UpdateRun", func(t *testing.T) {
		run, err := db.GetRun("run-001")
		if err != nil {
			t.Fatal(err)
		}

		newComb := 9.0
		run.JudgeCombined = &newComb
		reason := "max_turns"
		run.TerminationReason = &reason

		if err := db.UpdateRun(run); err != nil {
			t.Fatalf("update: %v", err)
		}

		got, err := db.GetRun("run-001")
		if err != nil {
			t.Fatal(err)
		}
		if got.JudgeCombined == nil || *got.JudgeCombined != 9.0 {
			t.Errorf("judge_combined = %v, want 9.0", got.JudgeCombined)
		}
		if got.TerminationReason == nil || *got.TerminationReason != "max_turns" {
			t.Errorf("termination_reason = %v, want max_turns", got.TerminationReason)
		}
	})

	t.Run("QueryByExperiment", func(t *testing.T) {
		// Insert additional runs
		for i, cond := range []string{"baseline", "aef-minimal"} {
			pass := false
			rate := 0.5
			run := &EvalRun{
				RunID:        fmt.Sprintf("run-00%d", i+2),
				Experiment:   "3A",
				Condition:    cond,
				TaskID:       "task-01",
				Attempt:      1,
				TestsPass:    &pass,
				TestPassRate: &rate,
				StartedAt:    now,
			}
			if err := db.InsertRun(run); err != nil {
				t.Fatalf("insert %s: %v", cond, err)
			}
		}

		// All conditions
		runs, err := db.QueryByExperiment("3A", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 3 {
			t.Errorf("got %d runs, want 3", len(runs))
		}

		// Filter by condition
		runs, err = db.QueryByExperiment("3A", "baseline")
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 1 {
			t.Errorf("got %d runs for baseline, want 1", len(runs))
		}
	})

	t.Run("RecallState", func(t *testing.T) {
		state := &EvalRecallState{
			RunID:        "run-001",
			ItemID:       "F-abc123",
			ItemType:     "failure",
			ItemTitle:    "Missing idempotency key",
			WasRetrieved: true,
			WasKept:      true,
			WasUsed:      false,
		}
		if err := db.InsertRecallState(state); err != nil {
			t.Fatalf("insert recall state: %v", err)
		}

		states, err := db.QueryRecallState("run-001")
		if err != nil {
			t.Fatal(err)
		}
		if len(states) != 1 {
			t.Fatalf("got %d states, want 1", len(states))
		}
		if !states[0].WasRetrieved {
			t.Error("was_retrieved = false, want true")
		}
		if states[0].WasUsed {
			t.Error("was_used = true, want false")
		}
	})

	t.Run("ComputeConditionStats", func(t *testing.T) {
		stats, err := db.ComputeConditionStats("3A")
		if err != nil {
			t.Fatal(err)
		}
		if len(stats) != 3 {
			t.Fatalf("got %d conditions, want 3", len(stats))
		}

		// Find aef-full stats
		var full *ConditionStats
		for i := range stats {
			if stats[i].Condition == "aef-full" {
				full = &stats[i]
				break
			}
		}
		if full == nil {
			t.Fatal("aef-full condition not found")
		}
		if full.RunCount != 1 {
			t.Errorf("run_count = %d, want 1", full.RunCount)
		}
	})
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"empty", nil, 0},
		{"one", []float64{5}, 5},
		{"odd", []float64{3, 1, 2}, 2},
		{"even", []float64{4, 1, 3, 2}, 2.5},
		{"already_sorted", []float64{1, 2, 3, 4, 5}, 3},
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
