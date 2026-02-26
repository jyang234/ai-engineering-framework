package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// PipeRunner executes evaluation runs using `claude -p` (pipe mode).
// Tests baseline and aef-minimal conditions.
type PipeRunner struct {
	resultsDB *ResultsDB
	taskDir   string // Path to task corpus directory
	logDir    string // Path to log output directory
}

// PipeRunConfig configures a single pipe-mode evaluation run.
// TaskInfo holds metadata about a discovered task in the corpus.
type TaskInfo struct {
	ID         string
	Complexity string
	Spec       string
	Pitfalls   []PitfallSpec
}

// PipeRunConfig configures a single pipe-mode evaluation run.
type PipeRunConfig struct {
	RunID      string // Optional: caller-assigned run ID (auto-generated if empty)
	Experiment string
	Condition  *Condition
	TaskID     string
	Complexity string
	Attempt    int
	Model      string // Claude model ID (default: claude-sonnet-4-6)
	MaxTokens  int    // Max output tokens per invocation
	TaskSpec   string // Task specification (README.md content)
	Pitfalls   []PitfallSpec
}

// NewPipeRunner creates a pipe-mode runner.
func NewPipeRunner(resultsDB *ResultsDB, taskDir, logDir string) *PipeRunner {
	return &PipeRunner{
		resultsDB: resultsDB,
		taskDir:   taskDir,
		logDir:    logDir,
	}
}

// RunTask executes a single task under a given condition using `claude -p`.
func (r *PipeRunner) RunTask(ctx context.Context, config *PipeRunConfig) (*EvalRun, error) {
	runID := fmt.Sprintf("%s-%s-%s-%d-%s",
		config.Experiment, config.Condition.Name, config.TaskID,
		config.Attempt, uuid.New().String()[:8])

	run := &EvalRun{
		RunID:          runID,
		Experiment:     config.Experiment,
		Condition:      string(config.Condition.Name),
		TaskID:         config.TaskID,
		TaskComplexity: config.Complexity,
		Attempt:        config.Attempt,
		StartedAt:      time.Now().UTC(),
	}

	// Insert the run record before execution
	if err := r.resultsDB.InsertRun(run); err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	// Set up isolated workspace
	workDir, err := r.setupWorkspace(config)
	if err != nil {
		return run, fmt.Errorf("setup workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Write system prompt file if needed
	promptFile, err := config.Condition.WriteSystemPromptFile(workDir)
	if err != nil {
		return run, fmt.Errorf("write prompt: %w", err)
	}

	// Execute claude -p
	log.Printf("[%s] Executing: claude -p (condition=%s, task=%s)", runID, config.Condition.Name, config.TaskID)
	startTime := time.Now()

	output, err := r.executeClaude(ctx, config, workDir, promptFile)
	duration := time.Since(startTime)
	durMs := duration.Milliseconds()
	run.DurationMs = &durMs

	// Log output
	r.writeLog(runID, output, err)

	if err != nil {
		reason := "claude_error"
		run.TerminationReason = &reason
		log.Printf("[%s] Claude execution error: %v", runID, err)
	}

	// Score the implementation
	log.Printf("[%s] Scoring...", runID)
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	scorer := NewScorer(apiKey, workDir)
	scoreResult, scoreErr := scorer.Score(ctx, config.TaskSpec, config.Pitfalls)
	if scoreErr != nil {
		log.Printf("[%s] Scoring error: %v", runID, scoreErr)
	} else {
		scorer.ApplyToRun(scoreResult, run)
	}

	// Update the run record
	if updateErr := r.resultsDB.UpdateRun(run); updateErr != nil {
		log.Printf("[%s] Update run error: %v", runID, updateErr)
	}

	log.Printf("[%s] Complete: tests_pass=%v judge_combined=%v duration=%v",
		runID, run.TestsPass, run.JudgeCombined, duration)

	return run, nil
}

// RunBatch executes all tasks in a corpus directory under a given condition.
func (r *PipeRunner) RunBatch(ctx context.Context, experiment string, condition *Condition, attempts int, model string) ([]*EvalRun, error) {
	tasks, err := r.discoverTasks()
	if err != nil {
		return nil, fmt.Errorf("discover tasks: %w", err)
	}

	log.Printf("Batch: %d tasks × %d attempts = %d runs (condition=%s)",
		len(tasks), attempts, len(tasks)*attempts, condition.Name)

	var runs []*EvalRun
	for _, task := range tasks {
		for attempt := 1; attempt <= attempts; attempt++ {
			select {
			case <-ctx.Done():
				return runs, ctx.Err()
			default:
			}

			config := &PipeRunConfig{
				Experiment: experiment,
				Condition:  condition,
				TaskID:     task.id,
				Complexity: task.complexity,
				Attempt:    attempt,
				Model:      model,
				TaskSpec:   task.spec,
				Pitfalls:   task.pitfalls,
			}

			run, err := r.RunTask(ctx, config)
			runs = append(runs, run)
			if err != nil {
				log.Printf("Task %s attempt %d failed: %v", task.id, attempt, err)
			}
		}
	}

	return runs, nil
}

func (r *PipeRunner) setupWorkspace(config *PipeRunConfig) (string, error) {
	workDir, err := os.MkdirTemp("", fmt.Sprintf("eval-%s-*", config.TaskID))
	if err != nil {
		return "", err
	}

	// Copy task files to workspace
	taskSrc := filepath.Join(r.taskDir, config.TaskID)
	if err := copyDir(taskSrc, workDir); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("copy task: %w", err)
	}

	return workDir, nil
}

func (r *PipeRunner) executeClaude(ctx context.Context, config *PipeRunConfig, workDir, promptFile string) (string, error) {
	// Build command arguments
	args := []string{"-p"}

	if promptFile != "" {
		args = append(args, "--append-system-prompt-file", promptFile)
	}

	// Build allowed tools argument
	if len(config.Condition.AllowedTools) > 0 {
		tools := ""
		for i, t := range config.Condition.AllowedTools {
			if i > 0 {
				tools += ","
			}
			tools += t
		}
		args = append(args, "--allowedTools", tools)
	}

	model := config.Model
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	args = append(args, "--model", model)

	// Determine timeout based on complexity
	timeout := complexityTimeout(config.Complexity)
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "claude", args...)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewBufferString(config.TaskSpec)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- STDERR ---\n" + stderr.String()
	}

	return output, err
}

func (r *PipeRunner) writeLog(runID, output string, execErr error) {
	if r.logDir == "" {
		return
	}

	logDir := filepath.Join(r.logDir, runID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create log dir: %v", err)
		return
	}

	_ = os.WriteFile(filepath.Join(logDir, "output.txt"), []byte(output), 0644)

	if execErr != nil {
		_ = os.WriteFile(filepath.Join(logDir, "error.txt"), []byte(execErr.Error()), 0644)
	}
}

type taskInfo struct {
	id         string
	complexity string
	spec       string
	pitfalls   []PitfallSpec
}

// DiscoverTasks returns the exported task list for use by the CLI.
func (r *PipeRunner) DiscoverTasks() ([]TaskInfo, error) {
	internal, err := r.discoverTasks()
	if err != nil {
		return nil, err
	}
	result := make([]TaskInfo, len(internal))
	for i, t := range internal {
		result[i] = TaskInfo{
			ID:         t.id,
			Complexity: t.complexity,
			Spec:       t.spec,
			Pitfalls:   t.pitfalls,
		}
	}
	return result, nil
}

func (r *PipeRunner) discoverTasks() ([]taskInfo, error) {
	entries, err := os.ReadDir(r.taskDir)
	if err != nil {
		return nil, err
	}

	var tasks []taskInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskID := entry.Name()
		specPath := filepath.Join(r.taskDir, taskID, "README.md")
		specData, err := os.ReadFile(specPath)
		if err != nil {
			log.Printf("Skip %s: no README.md", taskID)
			continue
		}

		task := taskInfo{
			id:   taskID,
			spec: string(specData),
		}

		// Read scoring.yaml for complexity
		scoringPath := filepath.Join(r.taskDir, taskID, "scoring.yaml")
		if data, err := os.ReadFile(scoringPath); err == nil {
			task.complexity = parseScoringComplexity(string(data))
		}

		// Read pitfalls.yaml
		pitfallsPath := filepath.Join(r.taskDir, taskID, "pitfalls.yaml")
		if data, err := os.ReadFile(pitfallsPath); err == nil {
			task.pitfalls = parsePitfalls(data)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// parseScoringComplexity extracts complexity from scoring.yaml content.
// Simple format: looks for "complexity: <value>" line.
func parseScoringComplexity(content string) string {
	for _, line := range splitLines(content) {
		if len(line) > 12 && line[:12] == "complexity: " {
			return line[12:]
		}
	}
	return "moderate" // default
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// parsePitfalls parses pitfalls from YAML or JSON format.
// Tries YAML first (the spec's canonical format), then falls back to JSON.
func parsePitfalls(data []byte) []PitfallSpec {
	pitfalls, err := ParsePitfallsYAML(data)
	if err == nil && len(pitfalls) > 0 {
		return pitfalls
	}
	var jsonPitfalls []PitfallSpec
	_ = json.Unmarshal(data, &jsonPitfalls)
	return jsonPitfalls
}

func complexityTimeout(complexity string) time.Duration {
	switch complexity {
	case "simple":
		return 5 * time.Minute
	case "complex":
		return 20 * time.Minute
	default: // moderate
		return 10 * time.Minute
	}
}

// copyDir recursively copies src directory contents to dst.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
