package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Scorer evaluates a completed implementation run.
type Scorer struct {
	anthropic *AnthropicClient
	taskDir   string // Directory containing the agent's implementation
}

// ScoreResult holds all scoring outputs for a single run.
type ScoreResult struct {
	TestsPass       bool            `json:"tests_pass"`
	TestPassRate    float64         `json:"test_pass_rate"`
	TestsRun        int             `json:"tests_run"`
	TestsFailed     int             `json:"tests_failed"`
	TestOutput      string          `json:"test_output"`
	LintViolations  int             `json:"lint_violations"`
	LintClean       bool            `json:"lint_clean"`
	LintOutput      string          `json:"lint_output"`
	PitfallsTotal   int             `json:"pitfalls_total"`
	PitfallsAvoided int             `json:"pitfalls_avoided"`
	PitfallDetails  []PitfallResult `json:"pitfall_details"`

	// LLM judge scores
	JudgeCorrectness      float64 `json:"judge_correctness"`
	JudgeCodeQuality      float64 `json:"judge_code_quality"`
	JudgePitfallAvoidance float64 `json:"judge_pitfall_avoidance"`
	JudgeCompleteness     float64 `json:"judge_completeness"`
	JudgeEfficiency       float64 `json:"judge_efficiency"`
	JudgeCombined         float64 `json:"judge_combined"`
	JudgeReasoning        string  `json:"judge_reasoning"`
}

// PitfallResult tracks whether a specific pitfall was avoided.
type PitfallResult struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Avoided     bool   `json:"avoided"`
	Evidence    string `json:"evidence"`
}

// CodeQualityJudgment is the expected JSON output from the code quality judge.
type CodeQualityJudgment struct {
	Correctness      float64 `json:"correctness"`
	CodeQuality      float64 `json:"code_quality"`
	PitfallAvoidance float64 `json:"pitfall_avoidance"`
	Completeness     float64 `json:"completeness"`
	Efficiency       float64 `json:"efficiency"`
	Reasoning        string  `json:"reasoning"`
}

// Quality score weights from the spec.
const (
	weightCorrectness      = 0.30
	weightCodeQuality      = 0.20
	weightPitfallAvoidance = 0.25
	weightCompleteness     = 0.15
	weightEfficiency       = 0.10
)

// NewScorer creates a scorer for a task implementation directory.
func NewScorer(apiKey string, taskDir string) *Scorer {
	return &Scorer{
		anthropic: NewAnthropicClient(apiKey),
		taskDir:   taskDir,
	}
}

// Score runs the full scoring pipeline: tests, lint, pitfalls, LLM judge.
func (s *Scorer) Score(ctx context.Context, taskSpec string, pitfalls []PitfallSpec) (*ScoreResult, error) {
	result := &ScoreResult{}

	// 1. Run tests
	testResult, err := s.runTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("run tests: %w", err)
	}
	result.TestsPass = testResult.pass
	result.TestPassRate = testResult.passRate
	result.TestsRun = testResult.run
	result.TestsFailed = testResult.failed
	result.TestOutput = testResult.output

	// 2. Run linter
	lintResult, err := s.runLint(ctx)
	if err != nil {
		return nil, fmt.Errorf("run lint: %w", err)
	}
	result.LintViolations = lintResult.violations
	result.LintClean = lintResult.violations == 0
	result.LintOutput = lintResult.output

	// 3. Check pitfalls
	result.PitfallsTotal = len(pitfalls)
	result.PitfallDetails = s.checkPitfalls(pitfalls)
	for _, p := range result.PitfallDetails {
		if p.Avoided {
			result.PitfallsAvoided++
		}
	}

	// 4. LLM judge
	judgeResult, err := s.judgeCodeQuality(ctx, taskSpec, result)
	if err != nil {
		// Judge failure is non-fatal — log but don't fail scoring
		result.JudgeReasoning = fmt.Sprintf("judge error: %v", err)
	} else {
		result.JudgeCorrectness = judgeResult.Correctness
		result.JudgeCodeQuality = judgeResult.CodeQuality
		result.JudgePitfallAvoidance = judgeResult.PitfallAvoidance
		result.JudgeCompleteness = judgeResult.Completeness
		result.JudgeEfficiency = judgeResult.Efficiency
		result.JudgeReasoning = judgeResult.Reasoning
		result.JudgeCombined = weightCorrectness*judgeResult.Correctness +
			weightCodeQuality*judgeResult.CodeQuality +
			weightPitfallAvoidance*judgeResult.PitfallAvoidance +
			weightCompleteness*judgeResult.Completeness +
			weightEfficiency*judgeResult.Efficiency
	}

	return result, nil
}

// ApplyToRun copies scoring results into an EvalRun record.
func (s *Scorer) ApplyToRun(result *ScoreResult, run *EvalRun) {
	run.TestsPass = &result.TestsPass
	run.TestPassRate = &result.TestPassRate
	run.LintViolations = &result.LintViolations
	run.LintClean = &result.LintClean
	run.PitfallsTotal = &result.PitfallsTotal
	run.PitfallsAvoided = &result.PitfallsAvoided
	run.JudgeCorrectness = &result.JudgeCorrectness
	run.JudgeCodeQuality = &result.JudgeCodeQuality
	run.JudgePitfallAvoidance = &result.JudgePitfallAvoidance
	run.JudgeCompleteness = &result.JudgeCompleteness
	run.JudgeEfficiency = &result.JudgeEfficiency
	run.JudgeCombined = &result.JudgeCombined
	now := time.Now().UTC()
	run.CompletedAt = &now
	run.ResultsJSON = ptrStr(FormatResultsJSON(run))
}

type testResult struct {
	pass     bool
	passRate float64
	run      int
	failed   int
	output   string
}

func (s *Scorer) runTests(ctx context.Context) (*testResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-count=1", "-json", "./...")
	cmd.Dir = s.taskDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &testResult{
		output: truncateOutput(stdout.String() + stderr.String()),
	}

	// Parse JSON test output to count pass/fail
	passed, failed := parseTestJSON(stdout.String())
	result.run = passed + failed
	result.failed = failed
	result.pass = err == nil && failed == 0

	if result.run > 0 {
		result.passRate = float64(passed) / float64(result.run)
	} else if err == nil {
		// No tests found but build succeeded
		result.pass = true
		result.passRate = 1.0
	}

	return result, nil
}

// parseTestJSON counts pass/fail from `go test -json` output.
func parseTestJSON(output string) (passed, failed int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Test == "" {
			continue // Skip package-level events
		}
		switch event.Action {
		case "pass":
			passed++
		case "fail":
			failed++
		}
	}
	return
}

type lintResult struct {
	violations int
	output     string
}

func (s *Scorer) runLint(ctx context.Context) (*lintResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "./...")
	cmd.Dir = s.taskDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // Non-zero exit expected when violations exist

	output := stdout.String() + stderr.String()
	violations := countLintViolations(output)

	return &lintResult{
		violations: violations,
		output:     truncateOutput(output),
	}, nil
}

// countLintViolations counts lines matching typical lint output pattern.
func countLintViolations(output string) int {
	// golangci-lint outputs "file:line:col: message (linter)" per violation
	pattern := regexp.MustCompile(`^\S+\.go:\d+:\d+:`)
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if pattern.MatchString(line) {
			count++
		}
	}
	return count
}

func (s *Scorer) checkPitfalls(pitfalls []PitfallSpec) []PitfallResult {
	results := make([]PitfallResult, 0, len(pitfalls))

	for _, p := range pitfalls {
		pr := PitfallResult{
			ID:          p.ID,
			Description: p.Description,
		}

		// Read all .go files in the task directory
		goFiles := collectGoFiles(s.taskDir)
		codeContent := strings.Join(goFiles, "\n")

		// Check if the anti-pattern (correct implementation) is present
		if p.AntiPattern != "" {
			re, err := regexp.Compile(p.AntiPattern)
			if err == nil && re.MatchString(codeContent) {
				pr.Avoided = true
				pr.Evidence = "anti-pattern matched: correct implementation detected"
			}
		}

		// Check if the pitfall pattern (bad implementation) is present
		if p.Pattern != "" && !pr.Avoided {
			re, err := regexp.Compile(p.Pattern)
			if err == nil && re.MatchString(codeContent) {
				pr.Avoided = false
				pr.Evidence = "pitfall pattern matched: implementation fell into trap"
			} else if err == nil {
				// Pitfall pattern not found — assume avoided
				pr.Avoided = true
				pr.Evidence = "pitfall pattern not found in code"
			}
		}

		// If neither pattern matched, default to "unknown"
		if pr.Evidence == "" {
			pr.Evidence = "no pattern check configured"
		}

		results = append(results, pr)
	}
	return results
}

func collectGoFiles(dir string) []string {
	var contents []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				contents = append(contents, string(data))
			}
		}
		return nil
	})
	return contents
}

const codeQualitySystemPrompt = `You are a code quality evaluator. Score the implementation on 5 dimensions, each 0-10.

CORRECTNESS (0-10): Does the implementation satisfy all requirements?
- 10: All tests pass, all requirements met
- 7: Most tests pass, minor gaps
- 4: Some tests pass, significant gaps
- 0: No tests pass

CODE QUALITY (0-10): Is the code well-structured and maintainable?
- 10: Clean, idiomatic Go, well-organized, good error handling
- 7: Generally clean with minor issues
- 4: Functional but messy
- 0: Spaghetti code

PITFALL AVOIDANCE (0-10): Did the implementation avoid known traps?
- 10: All known pitfalls avoided
- 5: Some pitfalls avoided, some hit
- 0: Fell into every trap

COMPLETENESS (0-10): Were all requirements addressed?
- 10: Every requirement implemented
- 7: Most requirements, minor omissions
- 4: Major requirements missing
- 0: Barely started

EFFICIENCY (0-10): Was the task completed without unnecessary work?
- 10: Direct path to solution, minimal wasted effort
- 7: Some exploration but converged quickly
- 4: Significant thrashing or over-engineering
- 0: Never converged

Return ONLY valid JSON:
{"correctness": N, "code_quality": N, "pitfall_avoidance": N, "completeness": N, "efficiency": N, "reasoning": "brief explanation"}`

func (s *Scorer) judgeCodeQuality(ctx context.Context, taskSpec string, score *ScoreResult) (*CodeQualityJudgment, error) {
	if s.anthropic.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	goFiles := collectGoFiles(s.taskDir)
	implementation := strings.Join(goFiles, "\n---\n")
	if len(implementation) > 50000 {
		implementation = implementation[:50000] + "\n... (truncated)"
	}

	userPrompt := fmt.Sprintf(`Task Specification:
%s

Test Results: %d passed, %d failed (pass rate: %.0f%%)
Lint Violations: %d
Pitfalls Avoided: %d/%d

Implementation:
%s

Score this implementation.`,
		taskSpec,
		score.TestsRun-score.TestsFailed, score.TestsFailed, score.TestPassRate*100,
		score.LintViolations,
		score.PitfallsAvoided, score.PitfallsTotal,
		implementation,
	)

	text, err := s.anthropic.RawJudge(ctx, codeQualitySystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned, "\n"); idx >= 0 {
			cleaned = cleaned[idx+1:]
		}
		if idx := strings.LastIndex(cleaned, "```"); idx >= 0 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var judgment CodeQualityJudgment
	if err := json.Unmarshal([]byte(cleaned), &judgment); err != nil {
		return nil, fmt.Errorf("parse code quality judgment: %w (raw: %s)", err, text)
	}
	return &judgment, nil
}

func truncateOutput(s string) string {
	const maxLen = 10000
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}

func ptrStr(s string) *string {
	return &s
}
