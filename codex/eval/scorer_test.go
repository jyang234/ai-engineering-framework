//go:build fts5

package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Scoring Pipeline Tests — Given-When-Then
// =============================================================================

// --- parseTestJSON ---

func TestParseTestJSON_GivenAllTestsPassing_ThenCorrectCounts(t *testing.T) {
	// Given: go test -json output with 3 passing tests
	output := `{"Action":"run","Test":"TestA"}
{"Action":"output","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"pass","Test":"TestA","Elapsed":0.01}
{"Action":"run","Test":"TestB"}
{"Action":"pass","Test":"TestB","Elapsed":0.02}
{"Action":"run","Test":"TestC"}
{"Action":"pass","Test":"TestC","Elapsed":0.01}
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.05}
`

	// When: we parse the output
	passed, failed := parseTestJSON(output)

	// Then: 3 passed, 0 failed (package-level "pass" is excluded)
	if passed != 3 {
		t.Errorf("passed = %d, want 3", passed)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestParseTestJSON_GivenMixedPassFail_ThenCorrectCounts(t *testing.T) {
	// Given: output with 2 passing and 1 failing test
	output := `{"Action":"pass","Test":"TestA"}
{"Action":"fail","Test":"TestB"}
{"Action":"pass","Test":"TestC"}
{"Action":"fail","Package":"example.com/pkg"}
`

	// When: we parse the output
	passed, failed := parseTestJSON(output)

	// Then: 2 passed, 1 failed
	if passed != 2 {
		t.Errorf("passed = %d, want 2", passed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

func TestParseTestJSON_GivenEmptyOutput_ThenZeroCounts(t *testing.T) {
	// Given: empty output (no tests)
	output := ""

	// When: we parse
	passed, failed := parseTestJSON(output)

	// Then: 0 passed, 0 failed
	if passed != 0 || failed != 0 {
		t.Errorf("expected 0/0, got %d/%d", passed, failed)
	}
}

func TestParseTestJSON_GivenNonJSONLines_ThenIgnored(t *testing.T) {
	// Given: output with invalid JSON mixed in
	output := `not json
{"Action":"pass","Test":"TestA"}
also not json
{"Action":"fail","Test":"TestB"}
`

	// When: we parse
	passed, failed := parseTestJSON(output)

	// Then: valid events are counted, invalid lines ignored
	if passed != 1 || failed != 1 {
		t.Errorf("expected 1/1, got %d/%d", passed, failed)
	}
}

func TestParseTestJSON_GivenPackageLevelEvents_ThenSkipped(t *testing.T) {
	// Given: only package-level events (no Test field)
	output := `{"Action":"pass","Package":"example.com/pkg"}
{"Action":"fail","Package":"example.com/other"}
`

	// When: we parse
	passed, failed := parseTestJSON(output)

	// Then: 0 counted (package-level events have empty Test)
	if passed != 0 || failed != 0 {
		t.Errorf("expected 0/0 for package-level only, got %d/%d", passed, failed)
	}
}

// --- countLintViolations ---

func TestCountLintViolations_GivenCleanOutput_ThenZeroViolations(t *testing.T) {
	// Given: golangci-lint output with no violations
	output := ""

	// When: we count violations
	count := countLintViolations(output)

	// Then: zero violations
	if count != 0 {
		t.Errorf("violations = %d, want 0", count)
	}
}

func TestCountLintViolations_GivenTypicalViolations_ThenCorrectCount(t *testing.T) {
	// Given: golangci-lint output with 3 violations
	output := `main.go:10:5: exported function Foo should have comment or be unexported (golint)
handler.go:25:12: shadow: declaration of "err" shadows declaration at line 20 (govet)
utils.go:42:1: unnecessary trailing newline (whitespace)
`

	// When: we count violations
	count := countLintViolations(output)

	// Then: 3 violations counted
	if count != 3 {
		t.Errorf("violations = %d, want 3", count)
	}
}

func TestCountLintViolations_GivenMixedOutputWithNonViolationLines_ThenOnlyViolationsCounted(t *testing.T) {
	// Given: output with violations and informational lines
	output := `Running golangci-lint...
main.go:10:5: some violation (linter)
level=warning msg="some warning"
utils.go:42:1: another violation (linter)
`

	// When: we count violations
	count := countLintViolations(output)

	// Then: only the 2 matching lines are counted
	if count != 2 {
		t.Errorf("violations = %d, want 2", count)
	}
}

// --- checkPitfalls ---

func TestCheckPitfalls_GivenAntiPatternMatches_ThenPitfallAvoided(t *testing.T) {
	// Given: a workspace with code that matches the anti-pattern (correct approach)
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main
import "math/rand"
func retry() {
	jitter := rand.Intn(1000) // adds jitter to avoid thundering herd
}
`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{
			ID:          "p-jitter",
			Description: "Missing jitter in retry backoff",
			Pattern:     `time\.Sleep\(.*backoff\)`, // bad: sleep without jitter
			AntiPattern: `jitter`,                    // good: uses jitter
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: the pitfall should be marked as avoided
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Avoided {
		t.Error("expected pitfall to be avoided (anti-pattern matched)")
	}
	if results[0].Evidence == "" {
		t.Error("evidence should be non-empty")
	}
}

func TestCheckPitfalls_GivenPitfallPatternMatches_ThenPitfallNotAvoided(t *testing.T) {
	// Given: code that matches the pitfall pattern (bad approach, no anti-pattern)
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main
import "time"
func retry() {
	time.Sleep(backoff) // no jitter — thundering herd risk
}
`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{
			ID:          "p-jitter",
			Description: "Missing jitter",
			Pattern:     `time\.Sleep\(backoff\)`, // bad: sleep without jitter
			AntiPattern: `rand.*jitter`,            // good: uses random jitter
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: the pitfall should NOT be marked as avoided
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Avoided {
		t.Error("expected pitfall NOT to be avoided (pitfall pattern matched)")
	}
}

func TestCheckPitfalls_GivenNeitherPatternMatches_ThenPitfallAssumedAvoided(t *testing.T) {
	// Given: code that matches neither the pitfall nor the anti-pattern
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main
func hello() { println("hello") }
`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{
			ID:      "p-jitter",
			Pattern: `thundering_herd_detected`,
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: pitfall assumed avoided (bad pattern not found)
	if !results[0].Avoided {
		t.Error("expected pitfall assumed avoided when bad pattern not found")
	}
}

func TestCheckPitfalls_GivenNoPatternsConfigured_ThenEvidenceSaysUnconfigured(t *testing.T) {
	// Given: a pitfall spec with no patterns
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{{ID: "p-empty", Description: "no patterns set"}}

	// When: we check
	results := scorer.checkPitfalls(pitfalls)

	// Then: evidence indicates no pattern check configured
	if results[0].Evidence != "no pattern check configured" {
		t.Errorf("evidence = %q, want 'no pattern check configured'", results[0].Evidence)
	}
}

func TestCheckPitfalls_GivenMultiplePitfalls_ThenEachEvaluatedIndependently(t *testing.T) {
	// Given: code that avoids pitfall 1 but hits pitfall 2
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main
func process() {
	jitter := 100 // good: has jitter
	// bad: no idempotency key
	db.Insert(payment)
}
`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{ID: "p1", AntiPattern: `jitter`},
		{ID: "p2", Pattern: `db\.Insert\(payment\)`, AntiPattern: `idempotencyKey`},
	}

	// When: we check both pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: p1 avoided (anti-pattern matched), p2 not avoided (pitfall matched)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Avoided {
		t.Error("p1 should be avoided (jitter found)")
	}
	if results[1].Avoided {
		t.Error("p2 should NOT be avoided (db.Insert without idempotency key)")
	}
}

func TestCheckPitfalls_GivenTestFiles_ThenExcludedFromScan(t *testing.T) {
	// Given: anti-pattern only exists in a test file (should be excluded)
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(`package main
func process() {}
`), 0644)
	os.WriteFile(filepath.Join(workDir, "main_test.go"), []byte(`package main
func TestProcess() { jitter := 100 }
`), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{{ID: "p1", AntiPattern: `jitter`}}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: the anti-pattern should NOT be found (test files excluded)
	if results[0].Avoided {
		t.Error("anti-pattern in test file should not count as avoided")
	}
}

// --- Quality Score Weights ---

func TestQualityWeights_SumToOne(t *testing.T) {
	// Given: the weight constants from the spec
	sum := weightCorrectness + weightCodeQuality + weightPitfallAvoidance +
		weightCompleteness + weightEfficiency

	// Then: they should sum to 1.0
	if sum != 1.0 {
		t.Errorf("weights sum = %v, want 1.0", sum)
	}
}

func TestQualityWeights_MatchSpecValues(t *testing.T) {
	// Given: the weight constants should match the spec's 0.30/0.20/0.25/0.15/0.10
	if weightCorrectness != 0.30 {
		t.Errorf("correctness weight = %v, want 0.30", weightCorrectness)
	}
	if weightCodeQuality != 0.20 {
		t.Errorf("code_quality weight = %v, want 0.20", weightCodeQuality)
	}
	if weightPitfallAvoidance != 0.25 {
		t.Errorf("pitfall_avoidance weight = %v, want 0.25", weightPitfallAvoidance)
	}
	if weightCompleteness != 0.15 {
		t.Errorf("completeness weight = %v, want 0.15", weightCompleteness)
	}
	if weightEfficiency != 0.10 {
		t.Errorf("efficiency weight = %v, want 0.10", weightEfficiency)
	}
}

// --- ApplyToRun ---

func TestApplyToRun_GivenScoreResult_WhenApplied_ThenRunPopulated(t *testing.T) {
	// Given: a score result with known values
	score := &ScoreResult{
		TestsPass: true, TestPassRate: 0.9, LintViolations: 2, LintClean: false,
		PitfallsTotal: 3, PitfallsAvoided: 2,
		JudgeCorrectness: 8.0, JudgeCodeQuality: 7.0, JudgePitfallAvoidance: 9.0,
		JudgeCompleteness: 8.0, JudgeEfficiency: 7.0, JudgeCombined: 8.0,
	}
	run := &EvalRun{RunID: "apply-001", Experiment: "3A", Condition: "baseline",
		TaskID: "task-01", Attempt: 1, StartedAt: time.Now().UTC()}

	scorer := &Scorer{}

	// When: we apply the score to the run
	scorer.ApplyToRun(score, run)

	// Then: all fields should be populated
	if run.TestsPass == nil || !*run.TestsPass {
		t.Error("TestsPass should be true")
	}
	if run.TestPassRate == nil || *run.TestPassRate != 0.9 {
		t.Errorf("TestPassRate = %v, want 0.9", run.TestPassRate)
	}
	if run.LintViolations == nil || *run.LintViolations != 2 {
		t.Errorf("LintViolations = %v, want 2", run.LintViolations)
	}
	if run.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if run.ResultsJSON == nil || *run.ResultsJSON == "" {
		t.Error("ResultsJSON should be populated")
	}
}

// --- truncateOutput ---

func TestTruncateOutput_GivenShortString_ThenUnchanged(t *testing.T) {
	result := truncateOutput("hello")
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestTruncateOutput_GivenLongString_ThenTruncated(t *testing.T) {
	// Given: a string longer than 10000 characters
	long := make([]byte, 15000)
	for i := range long {
		long[i] = 'x'
	}

	// When: truncated
	result := truncateOutput(string(long))

	// Then: length should be 10000 + truncation message
	if len(result) < 10000 || len(result) > 10020 {
		t.Errorf("truncated length = %d, expected ~10000+suffix", len(result))
	}
}

// --- collectGoFiles ---

func TestCollectGoFiles_GivenNestedDirectory_ThenAllGoFilesCollected(t *testing.T) {
	// Given: a directory tree with .go files at multiple levels
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "helper.go"), []byte("package sub"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "helper_test.go"), []byte("package sub"), 0644)

	// When: we collect go files
	files := collectGoFiles(dir)

	// Then: 2 files collected (test files excluded)
	if len(files) != 2 {
		t.Errorf("collected %d files, want 2 (excluding test files)", len(files))
	}
}

// =============================================================================
// Detection Method Tests — Given-When-Then (spec Gap 6)
// =============================================================================

// --- checkPitfallByGrep ---

func TestCheckPitfallByGrep_GivenDetectionFilesPattern_ThenOnlyMatchingFilesScanned(t *testing.T) {
	// Given: a workspace with .go and .yaml files, pitfall only in .yaml
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\nfunc hello() {}"), 0644)
	os.WriteFile(filepath.Join(workDir, "config.yaml"), []byte("retry_without_jitter: true"), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfall := PitfallSpec{
		ID:          "p-files",
		Description: "Check specific files",
		Detection: PitfallDetection{
			Method:  DetectionGrep,
			Pattern: `jitter`,
			Files:   []string{"*.yaml"},
		},
	}

	// When: we check using grep detection with file filter
	result := scorer.checkPitfallByGrep(pitfall)

	// Then: anti-pattern found in yaml file
	if !result.Avoided {
		t.Error("expected pitfall avoided (anti-pattern 'jitter' found in .yaml)")
	}
}

func TestCheckPitfallByGrep_GivenDetectionPatternOverridesAntiPattern_ThenDetectionPatternUsed(t *testing.T) {
	// Given: a pitfall with both Detection.Pattern and AntiPattern set
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\nfunc retry() { addJitter() }"), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfall := PitfallSpec{
		ID:          "p-override",
		Description: "Detection pattern overrides anti-pattern",
		AntiPattern: "nonexistent_pattern",
		Detection: PitfallDetection{
			Method:  DetectionGrep,
			Pattern: `addJitter`,
		},
	}

	// When: we check — Detection.Pattern should be used as anti-pattern
	result := scorer.checkPitfallByGrep(pitfall)

	// Then: avoided because Detection.Pattern matched
	if !result.Avoided {
		t.Errorf("expected avoided (Detection.Pattern should override), evidence: %s", result.Evidence)
	}
}

func TestCheckPitfallByGrep_GivenFallbackToTopLevelPatterns_ThenBothWork(t *testing.T) {
	// Given: pitfall with only top-level Pattern/AntiPattern (no Detection)
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\nimport \"time\"\nfunc f() { time.Sleep(backoff) }"), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfall := PitfallSpec{
		ID:      "p-fallback",
		Pattern: `time\.Sleep\(backoff\)`,
	}

	// When: we check using default grep detection
	result := scorer.checkPitfallByGrep(pitfall)

	// Then: pitfall NOT avoided (bad pattern matched)
	if result.Avoided {
		t.Error("expected pitfall NOT avoided (bad pattern matched)")
	}
}

// --- checkPitfallByTest ---

func TestCheckPitfallByTest_GivenEmptyTestName_ThenEvidenceExplainsNoTestName(t *testing.T) {
	// Given: a pitfall with test detection but empty pattern
	workDir := t.TempDir()
	scorer := &Scorer{taskDir: workDir}
	pitfall := PitfallSpec{
		ID:        "p-no-test",
		Detection: PitfallDetection{Method: DetectionTest, Pattern: ""},
	}

	// When: we check
	result := scorer.checkPitfallByTest(pitfall)

	// Then: evidence mentions no test name specified
	if result.Evidence != "test detection: no test name specified" {
		t.Errorf("evidence = %q", result.Evidence)
	}
}

// --- checkPitfallByJudge ---

func TestCheckPitfallByJudge_GivenNoAPIKey_WhenCheckingPitfall_ThenGracefulDegradation(t *testing.T) {
	// Given: a scorer with no API key
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	scorer := &Scorer{
		anthropic: NewAnthropicClient(""),
		taskDir:   workDir,
	}

	pitfalls := []PitfallSpec{
		{
			ID:        "p-judge",
			Detection: PitfallDetection{Method: DetectionJudge, Pattern: "Is jitter used?"},
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: judge detection is skipped gracefully
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Evidence != "judge detection skipped: no API key" {
		t.Errorf("evidence = %q, expected graceful skip message", results[0].Evidence)
	}
}

// --- Detection method dispatch ---

func TestCheckPitfalls_GivenGrepMethod_ThenDispatchesToGrep(t *testing.T) {
	// Given: a pitfall with explicit grep detection
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\nfunc f() { jitter := 100 }"), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{
			ID:        "p-grep",
			Detection: PitfallDetection{Method: DetectionGrep, Pattern: `jitter`},
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: detected via grep
	if !results[0].Avoided {
		t.Error("expected pitfall avoided via grep detection")
	}
}

func TestCheckPitfalls_GivenDefaultMethod_ThenFallsBackToGrep(t *testing.T) {
	// Given: a pitfall with no detection method set (empty string)
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\nfunc f() { addJitter() }"), 0644)

	scorer := &Scorer{taskDir: workDir}
	pitfalls := []PitfallSpec{
		{
			ID:          "p-default",
			AntiPattern: `addJitter`,
		},
	}

	// When: we check pitfalls
	results := scorer.checkPitfalls(pitfalls)

	// Then: falls back to grep detection
	if !results[0].Avoided {
		t.Error("expected pitfall avoided via default grep detection")
	}
}

// =============================================================================
// YAML Pitfall Parsing Tests — Given-When-Then
// =============================================================================

func TestParsePitfallsYAML_GivenValidYAML_ThenParsedCorrectly(t *testing.T) {
	// Given: valid YAML pitfalls (spec canonical format)
	yaml := `
- id: p-jitter
  type: failure
  title: "Thundering herd from retry without jitter"
  description: "Missing jitter in retry backoff"
  tags: ["retry", "backoff", "jitter"]
  detection:
    method: grep
    pattern: "rand|jitter|Jitter|randomize"
    files: ["*.go"]
- id: p-idempotency
  description: "No idempotency key"
  pattern: "db\\.Insert\\(payment\\)"
  anti_pattern: "idempotencyKey"
`

	// When: we parse
	pitfalls, err := ParsePitfallsYAML([]byte(yaml))

	// Then: 2 pitfalls with correct fields
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pitfalls) != 2 {
		t.Fatalf("got %d pitfalls, want 2", len(pitfalls))
	}

	p1 := pitfalls[0]
	if p1.ID != "p-jitter" {
		t.Errorf("p1 ID = %q", p1.ID)
	}
	if p1.Type != "failure" {
		t.Errorf("p1 Type = %q, want failure", p1.Type)
	}
	if p1.Detection.Method != DetectionGrep {
		t.Errorf("p1 Detection.Method = %q, want grep", p1.Detection.Method)
	}
	if len(p1.Tags) != 3 {
		t.Errorf("p1 Tags = %d, want 3", len(p1.Tags))
	}
	if len(p1.Detection.Files) != 1 || p1.Detection.Files[0] != "*.go" {
		t.Errorf("p1 Detection.Files = %v, want [*.go]", p1.Detection.Files)
	}

	p2 := pitfalls[1]
	if p2.Pattern != `db\.Insert\(payment\)` {
		t.Errorf("p2 Pattern = %q", p2.Pattern)
	}
	if p2.AntiPattern != "idempotencyKey" {
		t.Errorf("p2 AntiPattern = %q", p2.AntiPattern)
	}
}

func TestParsePitfallsYAML_GivenInvalidYAML_ThenErrorReturned(t *testing.T) {
	// Given: invalid YAML
	_, err := ParsePitfallsYAML([]byte("{{invalid yaml"))

	// Then: error returned
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParsePitfallsYAML_GivenEmptyYAML_ThenEmptySlice(t *testing.T) {
	// Given: empty YAML
	pitfalls, err := ParsePitfallsYAML([]byte(""))

	// Then: empty slice, no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pitfalls) != 0 {
		t.Errorf("got %d pitfalls, want 0", len(pitfalls))
	}
}

func TestParsePitfallsYAML_GivenSeedsInYAML_ThenSeedsParsed(t *testing.T) {
	// Given: YAML with RECALL seeds
	yaml := `
- id: p-seeds
  description: "Test seeds"
  seeds:
    - type: failure
      title: "Missing jitter"
      content: "Always add jitter to retry backoff"
      tags: ["retry"]
`

	// When: we parse
	pitfalls, err := ParsePitfallsYAML([]byte(yaml))

	// Then: seeds parsed correctly
	if err != nil {
		t.Fatal(err)
	}
	if len(pitfalls[0].Seeds) != 1 {
		t.Fatalf("seeds = %d, want 1", len(pitfalls[0].Seeds))
	}
	seed := pitfalls[0].Seeds[0]
	if seed.Type != "failure" {
		t.Errorf("seed type = %q", seed.Type)
	}
	if seed.Title != "Missing jitter" {
		t.Errorf("seed title = %q", seed.Title)
	}
}

// =============================================================================
// ProcessData Tests — Given-When-Then (spec Gap 7)
// =============================================================================

func TestProcessData_GivenInitialized_ThenFieldsAccessible(t *testing.T) {
	// Given: process data with known values
	pd := &ProcessData{
		TurnCount:     15,
		ActionSummary: "3 Edit calls, 2 failed test runs, 1 approach change",
	}

	// Then: fields are accessible
	if pd.TurnCount != 15 {
		t.Errorf("TurnCount = %d, want 15", pd.TurnCount)
	}
	if pd.ActionSummary != "3 Edit calls, 2 failed test runs, 1 approach change" {
		t.Errorf("ActionSummary = %q", pd.ActionSummary)
	}
}

// =============================================================================
// collectFilesByGlob Tests — Given-When-Then
// =============================================================================

func TestCollectFilesByGlob_GivenGoPattern_ThenOnlyGoFilesReturned(t *testing.T) {
	// Given: a directory with mixed file types
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(workDir, "config.yaml"), []byte("key: value"), 0644)
	os.WriteFile(filepath.Join(workDir, "main_test.go"), []byte("package main"), 0644)

	scorer := &Scorer{taskDir: workDir}

	// When: we collect with *.go pattern
	content := scorer.collectFilesByGlob([]string{"*.go"})

	// Then: only main.go content (test files excluded)
	if content != "package main" {
		t.Errorf("content = %q, want 'package main'", content)
	}
}

func TestCollectFilesByGlob_GivenMultiplePatterns_ThenAllMatched(t *testing.T) {
	// Given: a directory with .go and .yaml files
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(workDir, "config.yaml"), []byte("key: value"), 0644)

	scorer := &Scorer{taskDir: workDir}

	// When: we collect with multiple patterns
	content := scorer.collectFilesByGlob([]string{"*.go", "*.yaml"})

	// Then: both files' contents are included
	if !strings.Contains(content, "package main") {
		t.Error("missing .go content")
	}
	if !strings.Contains(content, "key: value") {
		t.Error("missing .yaml content")
	}
}
