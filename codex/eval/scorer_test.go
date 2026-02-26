//go:build fts5

package eval

import (
	"os"
	"path/filepath"
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
