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

	"gopkg.in/yaml.v3"
)

// ProcessData holds agent process metrics passed to the judge for efficiency scoring.
// Resolves spec Gap 7: efficiency dimension requires process data not in final code.
type ProcessData struct {
	TurnCount     int    `json:"turn_count"`
	ActionSummary string `json:"action_summary"` // e.g., "3 Edit calls, 2 failed test runs, 1 approach change"
}

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
	return s.ScoreWithProcess(ctx, taskSpec, pitfalls, nil)
}

// ScoreWithProcess runs the full scoring pipeline with optional process data for efficiency scoring.
func (s *Scorer) ScoreWithProcess(ctx context.Context, taskSpec string, pitfalls []PitfallSpec, process *ProcessData) (*ScoreResult, error) {
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

	// 3. Check pitfalls (supports grep, test, and judge detection methods per spec Gap 6)
	result.PitfallsTotal = len(pitfalls)
	result.PitfallDetails = s.checkPitfalls(pitfalls)
	for _, p := range result.PitfallDetails {
		if p.Avoided {
			result.PitfallsAvoided++
		}
	}

	// 4. LLM judge (includes process data for efficiency scoring per spec Gap 7)
	judgeResult, err := s.judgeCodeQuality(ctx, taskSpec, result, process)
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

		// Dispatch based on detection method (spec Gap 6: three detection methods)
		switch p.Detection.Method {
		case DetectionTest:
			pr = s.checkPitfallByTest(p)
		case DetectionJudge:
			// Judge-based detection requires LLM; degrade to grep if no API key
			if s.anthropic != nil && s.anthropic.apiKey != "" {
				pr = s.checkPitfallByJudge(p)
			} else {
				pr.Evidence = "judge detection skipped: no API key"
			}
		default:
			// Default to grep detection (including when method is empty or "grep")
			pr = s.checkPitfallByGrep(p)
		}

		results = append(results, pr)
	}
	return results
}

// checkPitfallByGrep uses regex pattern matching on source files.
func (s *Scorer) checkPitfallByGrep(p PitfallSpec) PitfallResult {
	pr := PitfallResult{
		ID:          p.ID,
		Description: p.Description,
	}

	// Determine which files to scan
	var codeContent string
	if len(p.Detection.Files) > 0 {
		codeContent = s.collectFilesByGlob(p.Detection.Files)
	} else {
		goFiles := collectGoFiles(s.taskDir)
		codeContent = strings.Join(goFiles, "\n")
	}

	// Use Detection.Pattern if set, fallback to top-level AntiPattern/Pattern
	antiPattern := p.AntiPattern
	pitfallPattern := p.Pattern
	if p.Detection.Method == DetectionGrep && p.Detection.Pattern != "" {
		// Spec convention: detection.pattern is the anti-pattern (what correct code looks like)
		antiPattern = p.Detection.Pattern
	}

	// Check if the anti-pattern (correct implementation) is present
	if antiPattern != "" {
		re, err := regexp.Compile(antiPattern)
		if err == nil && re.MatchString(codeContent) {
			pr.Avoided = true
			pr.Evidence = "anti-pattern matched: correct implementation detected"
			return pr
		}
	}

	// Check if the pitfall pattern (bad implementation) is present
	if pitfallPattern != "" {
		re, err := regexp.Compile(pitfallPattern)
		if err == nil && re.MatchString(codeContent) {
			pr.Avoided = false
			pr.Evidence = "pitfall pattern matched: implementation fell into trap"
			return pr
		} else if err == nil {
			pr.Avoided = true
			pr.Evidence = "pitfall pattern not found in code"
			return pr
		}
	}

	pr.Evidence = "no pattern check configured"
	return pr
}

// checkPitfallByTest checks if a specific test passes or fails.
func (s *Scorer) checkPitfallByTest(p PitfallSpec) PitfallResult {
	pr := PitfallResult{
		ID:          p.ID,
		Description: p.Description,
	}

	testName := p.Detection.Pattern
	if testName == "" {
		pr.Evidence = "test detection: no test name specified"
		return pr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-count=1", "-run", testName, "-json", "./...")
	cmd.Dir = s.taskDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	passed, failed := parseTestJSON(stdout.String())

	if err == nil && passed > 0 && failed == 0 {
		pr.Avoided = true
		pr.Evidence = fmt.Sprintf("test detection: %s passed (%d pass, %d fail)", testName, passed, failed)
	} else {
		pr.Avoided = false
		pr.Evidence = fmt.Sprintf("test detection: %s failed (%d pass, %d fail)", testName, passed, failed)
	}
	return pr
}

// checkPitfallByJudge asks an LLM judge whether the pitfall was avoided.
func (s *Scorer) checkPitfallByJudge(p PitfallSpec) PitfallResult {
	pr := PitfallResult{
		ID:          p.ID,
		Description: p.Description,
	}

	query := p.Detection.Pattern
	if query == "" {
		query = p.Description
	}

	goFiles := collectGoFiles(s.taskDir)
	code := strings.Join(goFiles, "\n---\n")
	if len(code) > 30000 {
		code = code[:30000] + "\n... (truncated)"
	}

	systemPrompt := `You are a code reviewer checking if a specific pitfall was avoided. Answer ONLY "avoided" or "hit" followed by a brief explanation.`
	userPrompt := fmt.Sprintf("Pitfall: %s\n\nQuery: %s\n\nCode:\n%s\n\nWas this pitfall avoided or hit?",
		p.Description, query, code)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, err := s.anthropic.RawJudge(ctx, systemPrompt, userPrompt)
	if err != nil {
		pr.Evidence = fmt.Sprintf("judge detection error: %v", err)
		return pr
	}

	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.HasPrefix(lower, "avoided") {
		pr.Avoided = true
		pr.Evidence = "judge detection: " + strings.TrimSpace(text)
	} else {
		pr.Avoided = false
		pr.Evidence = "judge detection: " + strings.TrimSpace(text)
	}
	return pr
}

// collectFilesByGlob collects file contents matching the given glob patterns.
func (s *Scorer) collectFilesByGlob(patterns []string) string {
	var contents []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(s.taskDir, pattern))
		if err != nil {
			continue
		}
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			data, err := os.ReadFile(path)
			if err == nil {
				contents = append(contents, string(data))
			}
		}
	}
	return strings.Join(contents, "\n")
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

func (s *Scorer) judgeCodeQuality(ctx context.Context, taskSpec string, score *ScoreResult, process *ProcessData) (*CodeQualityJudgment, error) {
	if s.anthropic.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	goFiles := collectGoFiles(s.taskDir)
	implementation := strings.Join(goFiles, "\n---\n")
	if len(implementation) > 50000 {
		implementation = implementation[:50000] + "\n... (truncated)"
	}

	// Build user prompt including process data for efficiency scoring (spec Gap 7)
	var processSection string
	if process != nil {
		processSection = fmt.Sprintf("\nProcess Data (for efficiency scoring):\nTurns taken: %d\nAction summary: %s\n",
			process.TurnCount, process.ActionSummary)
	} else {
		processSection = "\nProcess Data: Not available. Score efficiency based on code quality and apparent approach.\n"
	}

	userPrompt := fmt.Sprintf(`Task Specification:
%s

Test Results: %d passed, %d failed (pass rate: %.0f%%)
Lint Violations: %d
Pitfalls Avoided: %d/%d
%s
Implementation:
%s

Score this implementation.`,
		taskSpec,
		score.TestsRun-score.TestsFailed, score.TestsFailed, score.TestPassRate*100,
		score.LintViolations,
		score.PitfallsAvoided, score.PitfallsTotal,
		processSection,
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

// ParsePitfallsYAML parses pitfall specs from YAML format (the spec's canonical format).
func ParsePitfallsYAML(data []byte) ([]PitfallSpec, error) {
	var pitfalls []PitfallSpec
	if err := yaml.Unmarshal(data, &pitfalls); err != nil {
		return nil, fmt.Errorf("parse pitfalls YAML: %w", err)
	}
	return pitfalls, nil
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
