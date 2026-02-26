//go:build fts5

// aef-eval is the CLI tool for running AEF evaluation experiments.
//
// Usage:
//
//	aef-eval run    --experiment 3A --condition baseline --task-dir ./tasks --log-dir ./logs
//	aef-eval score  --run-id <id> --task-dir ./tasks
//	aef-eval report --experiment 3A --format text
//	aef-eval list   [--tasks | --runs | --experiments]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/codex/eval"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aef-eval",
		Short: "AEF Evaluation Framework CLI",
		Long:  "Run evaluation experiments, score results, and generate reports for the AI Engineering Framework.",
	}

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(scoreCmd())
	rootCmd.AddCommand(reportCmd())
	rootCmd.AddCommand(listCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// =============================================================================
// run command
// =============================================================================

func runCmd() *cobra.Command {
	var (
		experiment string
		condition  string
		taskDir    string
		logDir     string
		dbPath     string
		skillDir   string
		model      string
		strategy   string
		attempt    int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute evaluation runs",
		Long:  "Run evaluation experiments using pipe mode (Strategy B) or agent mode (Strategy C1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()

			// Validate required flags
			if experiment == "" {
				return fmt.Errorf("--experiment is required")
			}
			if taskDir == "" {
				return fmt.Errorf("--task-dir is required")
			}

			// Open results database
			if dbPath == "" {
				dbPath = filepath.Join(logDir, "results.db")
			}
			resultsDB, err := eval.OpenResultsDB(dbPath)
			if err != nil {
				return fmt.Errorf("open results db: %w", err)
			}
			defer resultsDB.Close()

			// Parse condition
			condName := eval.ConditionBaseline
			if condition != "" {
				condName, err = eval.ParseConditionName(condition)
				if err != nil {
					return err
				}
			}

			// Create condition
			cond, err := eval.NewCondition(condName, skillDir, nil)
			if err != nil {
				return fmt.Errorf("create condition: %w", err)
			}

			switch strategy {
			case "pipe", "B":
				return runPipeStrategy(ctx, resultsDB, cond, experiment, taskDir, logDir, model, attempt)
			case "agent", "C1":
				return runAgentStrategy(ctx, resultsDB, cond, experiment, taskDir, logDir, model, attempt)
			default:
				return fmt.Errorf("unknown strategy %q (use 'pipe' or 'agent')", strategy)
			}
		},
	}

	cmd.Flags().StringVar(&experiment, "experiment", "", "Experiment ID (e.g., 3A, 3B, 3C)")
	cmd.Flags().StringVar(&condition, "condition", "baseline", "Condition name (baseline, aef-minimal, aef-full)")
	cmd.Flags().StringVar(&taskDir, "task-dir", "", "Path to task corpus directory")
	cmd.Flags().StringVar(&logDir, "log-dir", "./eval-logs", "Path to log output directory")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to results database (default: <log-dir>/results.db)")
	cmd.Flags().StringVar(&skillDir, "skill-dir", "", "Path to skill files directory (required for aef-minimal/aef-full)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (default: claude-sonnet-4-6)")
	cmd.Flags().StringVar(&strategy, "strategy", "pipe", "Execution strategy: pipe (B) or agent (C1)")
	cmd.Flags().IntVar(&attempt, "attempt", 1, "Attempt number for repeated trials")

	return cmd
}

func runPipeStrategy(ctx context.Context, db *eval.ResultsDB, cond *eval.Condition, experiment, taskDir, logDir, model string, attempt int) error {
	runner := eval.NewPipeRunner(db, taskDir, logDir)

	// Discover tasks
	tasks, err := runner.DiscoverTasks()
	if err != nil {
		return fmt.Errorf("discover tasks: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found in %s", taskDir)
	}

	fmt.Fprintf(os.Stderr, "Discovered %d tasks in %s\n", len(tasks), taskDir)
	fmt.Fprintf(os.Stderr, "Running experiment %s with condition %s (attempt %d)\n", experiment, cond.Name, attempt)

	for i, task := range tasks {
		runID := fmt.Sprintf("%s-%s-%s-%d-%s", experiment, cond.Name, task.ID, attempt, uuid.New().String()[:8])
		fmt.Fprintf(os.Stderr, "[%d/%d] Task %s (run %s)...\n", i+1, len(tasks), task.ID, runID)

		config := &eval.PipeRunConfig{
			RunID:      runID,
			Experiment: experiment,
			Condition:  cond,
			TaskID:     task.ID,
			Complexity: task.Complexity,
			TaskSpec:   task.Spec,
			Pitfalls:   task.Pitfalls,
			Model:      model,
			Attempt:    attempt,
		}

		run, err := runner.RunTask(ctx, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
			continue
		}

		passRate := "N/A"
		if run.TestPassRate != nil {
			passRate = fmt.Sprintf("%.0f%%", *run.TestPassRate*100)
		}
		combined := "N/A"
		if run.JudgeCombined != nil {
			combined = fmt.Sprintf("%.1f", *run.JudgeCombined)
		}
		fmt.Fprintf(os.Stderr, "  Done: test_pass=%s judge_combined=%s\n", passRate, combined)
	}

	fmt.Fprintf(os.Stderr, "\nAll tasks complete. Results stored in %s\n", db.Path())
	return nil
}

func runAgentStrategy(ctx context.Context, db *eval.ResultsDB, cond *eval.Condition, experiment, taskDir, logDir, model string, attempt int) error {
	runner := eval.NewAgentRunner(db, taskDir, logDir)

	tasks, err := runner.DiscoverTasks()
	if err != nil {
		return fmt.Errorf("discover tasks: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found in %s", taskDir)
	}

	fmt.Fprintf(os.Stderr, "Discovered %d tasks in %s\n", len(tasks), taskDir)
	fmt.Fprintf(os.Stderr, "Running experiment %s with condition %s via agent (attempt %d)\n", experiment, cond.Name, attempt)

	for i, task := range tasks {
		runID := fmt.Sprintf("%s-%s-%s-%d-%s", experiment, cond.Name, task.ID, attempt, uuid.New().String()[:8])
		fmt.Fprintf(os.Stderr, "[%d/%d] Task %s (run %s)...\n", i+1, len(tasks), task.ID, runID)

		config := &eval.AgentRunConfig{
			RunID:      runID,
			Experiment: experiment,
			Condition:  cond,
			TaskID:     task.ID,
			Complexity: task.Complexity,
			TaskSpec:   task.Spec,
			Pitfalls:   task.Pitfalls,
			Model:      model,
			Attempt:    attempt,
		}

		run, err := runner.RunAgent(ctx, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
			continue
		}

		combined := "N/A"
		if run.JudgeCombined != nil {
			combined = fmt.Sprintf("%.1f", *run.JudgeCombined)
		}
		fmt.Fprintf(os.Stderr, "  Done: judge_combined=%s\n", combined)
	}

	fmt.Fprintf(os.Stderr, "\nAll tasks complete. Results stored in %s\n", db.Path())
	return nil
}

// =============================================================================
// score command
// =============================================================================

func scoreCmd() *cobra.Command {
	var (
		runID   string
		taskDir string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Score a completed evaluation run",
		Long:  "Run the scoring pipeline (tests, lint, pitfalls, LLM judge) against a completed run.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			if runID == "" {
				return fmt.Errorf("--run-id is required")
			}
			if taskDir == "" {
				return fmt.Errorf("--task-dir is required")
			}

			resultsDB, err := eval.OpenResultsDB(dbPath)
			if err != nil {
				return fmt.Errorf("open results db: %w", err)
			}
			defer resultsDB.Close()

			run, err := resultsDB.GetRun(runID)
			if err != nil {
				return fmt.Errorf("get run %s: %w", runID, err)
			}

			apiKey := os.Getenv("ANTHROPIC_API_KEY")
			scorer := eval.NewScorer(apiKey, taskDir)

			result, err := scorer.Score(ctx, "", nil)
			if err != nil {
				return fmt.Errorf("scoring: %w", err)
			}

			scorer.ApplyToRun(result, run)
			if err := resultsDB.UpdateRun(run); err != nil {
				return fmt.Errorf("update run: %w", err)
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to score")
	cmd.Flags().StringVar(&taskDir, "task-dir", "", "Path to task workspace")
	cmd.Flags().StringVar(&dbPath, "db", "./eval-logs/results.db", "Path to results database")

	return cmd
}

// =============================================================================
// report command
// =============================================================================

func reportCmd() *cobra.Command {
	var (
		experiment string
		format     string
		dbPath     string
		all        bool
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate evaluation reports",
		Long:  "Generate condition comparison tables, claim validation summaries, and scorecards.",
		RunE: func(cmd *cobra.Command, args []string) error {
			resultsDB, err := eval.OpenResultsDB(dbPath)
			if err != nil {
				return fmt.Errorf("open results db: %w", err)
			}
			defer resultsDB.Close()

			gen := eval.NewL3ReportGenerator(resultsDB)

			if all {
				return generateAllReports(gen, format)
			}

			if experiment == "" {
				return fmt.Errorf("--experiment or --all is required")
			}

			report, err := gen.GenerateExperimentReport(experiment)
			if err != nil {
				return fmt.Errorf("generate report: %w", err)
			}

			return outputReport(report, format)
		},
	}

	cmd.Flags().StringVar(&experiment, "experiment", "", "Experiment ID to report on")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&dbPath, "db", "./eval-logs/results.db", "Path to results database")
	cmd.Flags().BoolVar(&all, "all", false, "Generate reports for all experiments")

	return cmd
}

func generateAllReports(gen *eval.L3ReportGenerator, format string) error {
	experiments := []string{"3A", "3B", "3C", "3D"}
	var reports []*eval.ExperimentReport

	for _, exp := range experiments {
		report, err := gen.GenerateExperimentReport(exp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: experiment %s: %v\n", exp, err)
			continue
		}
		if len(report.Conditions) > 0 {
			reports = append(reports, report)
			if format == "text" {
				fmt.Println(eval.FormatExperimentReport(report))
			}
		}
	}

	if format == "json" {
		data, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else if format == "text" && len(reports) > 0 {
		fmt.Println(eval.FormatScorecard(reports))
	}

	return nil
}

func outputReport(report *eval.ExperimentReport, format string) error {
	switch format {
	case "json":
		out, err := eval.FormatExperimentReportJSON(report)
		if err != nil {
			return err
		}
		fmt.Println(out)
	default:
		fmt.Println(eval.FormatExperimentReport(report))
	}
	return nil
}

// =============================================================================
// list command
// =============================================================================

func listCmd() *cobra.Command {
	var (
		tasks       bool
		runs        bool
		experiments bool
		dbPath      string
		taskDir     string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks, runs, or experiments",
		RunE: func(cmd *cobra.Command, args []string) error {
			if tasks {
				return listTasks(taskDir)
			}
			if runs || experiments {
				return listRuns(dbPath, experiments)
			}
			return fmt.Errorf("specify --tasks, --runs, or --experiments")
		},
	}

	cmd.Flags().BoolVar(&tasks, "tasks", false, "List available tasks in the corpus")
	cmd.Flags().BoolVar(&runs, "runs", false, "List completed runs")
	cmd.Flags().BoolVar(&experiments, "experiments", false, "List experiments with run counts")
	cmd.Flags().StringVar(&dbPath, "db", "./eval-logs/results.db", "Path to results database")
	cmd.Flags().StringVar(&taskDir, "task-dir", "./tasks", "Path to task corpus directory")

	return cmd
}

func listTasks(taskDir string) error {
	runner := eval.NewPipeRunner(nil, taskDir, "")
	tasks, err := runner.DiscoverTasks()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks found in", taskDir)
		return nil
	}

	fmt.Printf("%-20s %-12s %s\n", "Task ID", "Complexity", "Pitfalls")
	fmt.Println(strings.Repeat("-", 50))
	for _, t := range tasks {
		fmt.Printf("%-20s %-12s %d\n", t.ID, t.Complexity, len(t.Pitfalls))
	}
	return nil
}

func listRuns(dbPath string, experimentsOnly bool) error {
	db, err := eval.OpenResultsDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if experimentsOnly {
		stats, err := db.ListExperiments()
		if err != nil {
			return err
		}
		fmt.Printf("%-12s %6s\n", "Experiment", "Runs")
		fmt.Println(strings.Repeat("-", 20))
		for _, s := range stats {
			fmt.Printf("%-12s %6d\n", s.Experiment, s.RunCount)
		}
		return nil
	}

	allRuns, err := db.QueryByExperiment("", "")
	if err != nil {
		return err
	}
	fmt.Printf("%-30s %-8s %-15s %-10s %10s\n", "Run ID", "Exp", "Condition", "Task", "Combined")
	fmt.Println(strings.Repeat("-", 80))
	for _, r := range allRuns {
		combined := "N/A"
		if r.JudgeCombined != nil {
			combined = fmt.Sprintf("%.1f", *r.JudgeCombined)
		}
		fmt.Printf("%-30s %-8s %-15s %-10s %10s\n",
			truncateStr(r.RunID, 30), r.Experiment, r.Condition, r.TaskID, combined)
	}
	return nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
