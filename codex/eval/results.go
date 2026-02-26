package eval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ResultsDB provides access to the eval results database.
type ResultsDB struct {
	db *sql.DB
}

// EvalRun represents a single evaluation run stored in the results database.
type EvalRun struct {
	RunID          string `json:"run_id"`
	Experiment     string `json:"experiment"` // "3A", "3B", "4A", etc.
	Condition      string `json:"condition"`  // "baseline", "aef-minimal", "aef-full"
	TaskID         string `json:"task_id"`
	TaskComplexity string `json:"task_complexity"` // "simple", "moderate", "complex"
	Attempt        int    `json:"attempt"`

	// Automated metrics
	TestsPass       *bool    `json:"tests_pass"`
	TestPassRate    *float64 `json:"test_pass_rate"`
	LintViolations  *int     `json:"lint_violations"`
	LintClean       *bool    `json:"lint_clean"`
	PitfallsTotal   *int     `json:"pitfalls_total"`
	PitfallsAvoided *int     `json:"pitfalls_avoided"`
	TurnsToComplete *int     `json:"turns_to_completion"`
	TokensConsumed  *int     `json:"tokens_consumed"`

	// LLM judge scores
	JudgeCorrectness      *float64 `json:"judge_correctness"`
	JudgeCodeQuality      *float64 `json:"judge_code_quality"`
	JudgePitfallAvoidance *float64 `json:"judge_pitfall_avoidance"`
	JudgeCompleteness     *float64 `json:"judge_completeness"`
	JudgeEfficiency       *float64 `json:"judge_efficiency"`
	JudgeCombined         *float64 `json:"judge_combined"`

	// Timing
	DurationMs  *int64     `json:"duration_ms"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Termination
	TerminationReason *string `json:"termination_reason,omitempty"`

	// Raw data
	TranscriptPath *string `json:"transcript_path"`
	ResultsJSON    *string `json:"results_json"`
}

// EvalRecallState tracks RECALL item usage during a run.
type EvalRecallState struct {
	RunID        string `json:"run_id"`
	ItemID       string `json:"item_id"`
	ItemType     string `json:"item_type"`
	ItemTitle    string `json:"item_title"`
	WasRetrieved bool   `json:"was_retrieved"`
	WasKept      bool   `json:"was_kept"`
	WasUsed      bool   `json:"was_used"`
}

// ConditionStats holds aggregated metrics for a single condition.
type ConditionStats struct {
	Condition           string  `json:"condition"`
	RunCount            int     `json:"run_count"`
	MedianTestPassRate  float64 `json:"median_test_pass_rate"`
	MedianLintClean     float64 `json:"median_lint_clean"`
	MedianPitfallRate   float64 `json:"median_pitfall_avoidance_rate"`
	MedianJudgeCombined float64 `json:"median_judge_combined"`
	MedianTurns         float64 `json:"median_turns"`
	MedianDurationMs    float64 `json:"median_duration_ms"`
}

// OpenResultsDB opens or creates the results database at the given path.
func OpenResultsDB(dbPath string) (*ResultsDB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create results dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open results db: %w", err)
	}

	if err := migrateResultsDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate results db: %w", err)
	}

	return &ResultsDB{db: db}, nil
}

// Close closes the database connection.
func (r *ResultsDB) Close() error {
	return r.db.Close()
}

func migrateResultsDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS eval_runs (
			run_id TEXT PRIMARY KEY,
			experiment TEXT NOT NULL,
			condition TEXT NOT NULL,
			task_id TEXT NOT NULL,
			task_complexity TEXT,
			attempt INTEGER DEFAULT 1,

			tests_pass BOOLEAN,
			test_pass_rate REAL,
			lint_violations INTEGER,
			lint_clean BOOLEAN,
			pitfalls_total INTEGER,
			pitfalls_avoided INTEGER,
			turns_to_completion INTEGER,
			tokens_consumed INTEGER,

			judge_correctness REAL,
			judge_code_quality REAL,
			judge_pitfall_avoidance REAL,
			judge_completeness REAL,
			judge_efficiency REAL,
			judge_combined REAL,

			duration_ms INTEGER,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,

			termination_reason TEXT,

			transcript_path TEXT,
			results_json TEXT
		);

		CREATE TABLE IF NOT EXISTS eval_recall_state (
			run_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			item_type TEXT,
			item_title TEXT,
			was_retrieved BOOLEAN,
			was_kept BOOLEAN,
			was_used BOOLEAN,
			PRIMARY KEY (run_id, item_id),
			FOREIGN KEY (run_id) REFERENCES eval_runs(run_id)
		);

		CREATE INDEX IF NOT EXISTS idx_eval_runs_experiment ON eval_runs(experiment);
		CREATE INDEX IF NOT EXISTS idx_eval_runs_condition ON eval_runs(condition);
		CREATE INDEX IF NOT EXISTS idx_eval_runs_task ON eval_runs(task_id);
	`)
	return err
}

// InsertRun inserts a new evaluation run.
func (r *ResultsDB) InsertRun(run *EvalRun) error {
	_, err := r.db.Exec(`
		INSERT INTO eval_runs (
			run_id, experiment, condition, task_id, task_complexity, attempt,
			tests_pass, test_pass_rate, lint_violations, lint_clean,
			pitfalls_total, pitfalls_avoided, turns_to_completion, tokens_consumed,
			judge_correctness, judge_code_quality, judge_pitfall_avoidance,
			judge_completeness, judge_efficiency, judge_combined,
			duration_ms, started_at, completed_at, termination_reason,
			transcript_path, results_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.RunID, run.Experiment, run.Condition, run.TaskID, run.TaskComplexity, run.Attempt,
		run.TestsPass, run.TestPassRate, run.LintViolations, run.LintClean,
		run.PitfallsTotal, run.PitfallsAvoided, run.TurnsToComplete, run.TokensConsumed,
		run.JudgeCorrectness, run.JudgeCodeQuality, run.JudgePitfallAvoidance,
		run.JudgeCompleteness, run.JudgeEfficiency, run.JudgeCombined,
		run.DurationMs, run.StartedAt, run.CompletedAt, run.TerminationReason,
		run.TranscriptPath, run.ResultsJSON,
	)
	return err
}

// UpdateRun updates an existing run with scoring results.
func (r *ResultsDB) UpdateRun(run *EvalRun) error {
	_, err := r.db.Exec(`
		UPDATE eval_runs SET
			tests_pass = ?, test_pass_rate = ?, lint_violations = ?, lint_clean = ?,
			pitfalls_total = ?, pitfalls_avoided = ?, turns_to_completion = ?, tokens_consumed = ?,
			judge_correctness = ?, judge_code_quality = ?, judge_pitfall_avoidance = ?,
			judge_completeness = ?, judge_efficiency = ?, judge_combined = ?,
			duration_ms = ?, completed_at = ?, termination_reason = ?,
			transcript_path = ?, results_json = ?
		WHERE run_id = ?`,
		run.TestsPass, run.TestPassRate, run.LintViolations, run.LintClean,
		run.PitfallsTotal, run.PitfallsAvoided, run.TurnsToComplete, run.TokensConsumed,
		run.JudgeCorrectness, run.JudgeCodeQuality, run.JudgePitfallAvoidance,
		run.JudgeCompleteness, run.JudgeEfficiency, run.JudgeCombined,
		run.DurationMs, run.CompletedAt, run.TerminationReason,
		run.TranscriptPath, run.ResultsJSON,
		run.RunID,
	)
	return err
}

// GetRun retrieves a single run by ID.
func (r *ResultsDB) GetRun(runID string) (*EvalRun, error) {
	row := r.db.QueryRow(`SELECT
		run_id, experiment, condition, task_id, task_complexity, attempt,
		tests_pass, test_pass_rate, lint_violations, lint_clean,
		pitfalls_total, pitfalls_avoided, turns_to_completion, tokens_consumed,
		judge_correctness, judge_code_quality, judge_pitfall_avoidance,
		judge_completeness, judge_efficiency, judge_combined,
		duration_ms, started_at, completed_at, termination_reason,
		transcript_path, results_json
		FROM eval_runs WHERE run_id = ?`, runID)
	return scanRun(row)
}

// QueryByExperiment returns all runs for an experiment, optionally filtered by condition.
func (r *ResultsDB) QueryByExperiment(experiment string, condition string) ([]*EvalRun, error) {
	query := `SELECT
		run_id, experiment, condition, task_id, task_complexity, attempt,
		tests_pass, test_pass_rate, lint_violations, lint_clean,
		pitfalls_total, pitfalls_avoided, turns_to_completion, tokens_consumed,
		judge_correctness, judge_code_quality, judge_pitfall_avoidance,
		judge_completeness, judge_efficiency, judge_combined,
		duration_ms, started_at, completed_at, termination_reason,
		transcript_path, results_json
		FROM eval_runs WHERE experiment = ?`
	args := []interface{}{experiment}

	if condition != "" {
		query += " AND condition = ?"
		args = append(args, condition)
	}
	query += " ORDER BY started_at"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*EvalRun
	for rows.Next() {
		run, err := scanRunFromRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// ComputeConditionStats computes aggregated statistics per condition for an experiment.
func (r *ResultsDB) ComputeConditionStats(experiment string) ([]ConditionStats, error) {
	runs, err := r.QueryByExperiment(experiment, "")
	if err != nil {
		return nil, err
	}

	byCondition := make(map[string][]*EvalRun)
	for _, run := range runs {
		byCondition[run.Condition] = append(byCondition[run.Condition], run)
	}

	var stats []ConditionStats
	for cond, condRuns := range byCondition {
		s := ConditionStats{
			Condition: cond,
			RunCount:  len(condRuns),
		}

		var testRates, judgeCombined, turns, durations []float64
		var lintCleanCount int
		var pitfallNums, pitfallDens []int

		for _, run := range condRuns {
			if run.TestPassRate != nil {
				testRates = append(testRates, *run.TestPassRate)
			}
			if run.LintClean != nil && *run.LintClean {
				lintCleanCount++
			}
			if run.PitfallsTotal != nil && run.PitfallsAvoided != nil {
				pitfallNums = append(pitfallNums, *run.PitfallsAvoided)
				pitfallDens = append(pitfallDens, *run.PitfallsTotal)
			}
			if run.JudgeCombined != nil {
				judgeCombined = append(judgeCombined, *run.JudgeCombined)
			}
			if run.TurnsToComplete != nil {
				turns = append(turns, float64(*run.TurnsToComplete))
			}
			if run.DurationMs != nil {
				durations = append(durations, float64(*run.DurationMs))
			}
		}

		s.MedianTestPassRate = median(testRates)
		s.MedianLintClean = float64(lintCleanCount) / float64(len(condRuns))
		if len(pitfallNums) > 0 {
			var rates []float64
			for i := range pitfallNums {
				if pitfallDens[i] > 0 {
					rates = append(rates, float64(pitfallNums[i])/float64(pitfallDens[i]))
				}
			}
			s.MedianPitfallRate = median(rates)
		}
		s.MedianJudgeCombined = median(judgeCombined)
		s.MedianTurns = median(turns)
		s.MedianDurationMs = median(durations)

		stats = append(stats, s)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Condition < stats[j].Condition
	})
	return stats, nil
}

// InsertRecallState records RECALL item tracking for a run.
func (r *ResultsDB) InsertRecallState(state *EvalRecallState) error {
	_, err := r.db.Exec(`
		INSERT OR REPLACE INTO eval_recall_state
		(run_id, item_id, item_type, item_title, was_retrieved, was_kept, was_used)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		state.RunID, state.ItemID, state.ItemType, state.ItemTitle,
		state.WasRetrieved, state.WasKept, state.WasUsed,
	)
	return err
}

// QueryRecallState returns RECALL state for a run.
func (r *ResultsDB) QueryRecallState(runID string) ([]*EvalRecallState, error) {
	rows, err := r.db.Query(`
		SELECT run_id, item_id, item_type, item_title, was_retrieved, was_kept, was_used
		FROM eval_recall_state WHERE run_id = ?`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*EvalRecallState
	for rows.Next() {
		s := &EvalRecallState{}
		if err := rows.Scan(&s.RunID, &s.ItemID, &s.ItemType, &s.ItemTitle,
			&s.WasRetrieved, &s.WasKept, &s.WasUsed); err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	return states, rows.Err()
}

// FormatResultsJSON serializes the run to JSON for the results_json column.
func FormatResultsJSON(run *EvalRun) string {
	data, _ := json.MarshalIndent(run, "", "  ")
	return string(data)
}

// scannable abstracts *sql.Row and *sql.Rows for shared scanning logic.
type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRunFields(s scannable) (*EvalRun, error) {
	run := &EvalRun{}
	err := s.Scan(
		&run.RunID, &run.Experiment, &run.Condition, &run.TaskID, &run.TaskComplexity, &run.Attempt,
		&run.TestsPass, &run.TestPassRate, &run.LintViolations, &run.LintClean,
		&run.PitfallsTotal, &run.PitfallsAvoided, &run.TurnsToComplete, &run.TokensConsumed,
		&run.JudgeCorrectness, &run.JudgeCodeQuality, &run.JudgePitfallAvoidance,
		&run.JudgeCompleteness, &run.JudgeEfficiency, &run.JudgeCombined,
		&run.DurationMs, &run.StartedAt, &run.CompletedAt, &run.TerminationReason,
		&run.TranscriptPath, &run.ResultsJSON,
	)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func scanRun(row *sql.Row) (*EvalRun, error) {
	return scanRunFields(row)
}

func scanRunFromRows(rows *sql.Rows) (*EvalRun, error) {
	return scanRunFields(rows)
}

// median computes the median of a float64 slice. Returns 0 for empty input.
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
