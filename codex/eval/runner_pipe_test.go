package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Pipe Runner Tests — Given-When-Then
// =============================================================================

// --- parseScoringComplexity ---

func TestParseScoringComplexity_GivenSimpleComplexity_ThenReturnsSimple(t *testing.T) {
	// Given: scoring.yaml content with "complexity: simple"
	content := "name: payment-retry\ncomplexity: simple\nmax_score: 100\n"

	// When: we parse
	result := parseScoringComplexity(content)

	// Then: returns "simple"
	if result != "simple" {
		t.Errorf("got %q, want %q", result, "simple")
	}
}

func TestParseScoringComplexity_GivenComplexComplexity_ThenReturnsComplex(t *testing.T) {
	// Given: scoring.yaml content with "complexity: complex"
	content := "complexity: complex\n"

	// When: we parse
	result := parseScoringComplexity(content)

	// Then: returns "complex"
	if result != "complex" {
		t.Errorf("got %q, want %q", result, "complex")
	}
}

func TestParseScoringComplexity_GivenNoComplexityLine_ThenDefaultsToModerate(t *testing.T) {
	// Given: scoring.yaml without a complexity line
	content := "name: task-01\nmax_score: 100\n"

	// When: we parse
	result := parseScoringComplexity(content)

	// Then: defaults to "moderate"
	if result != "moderate" {
		t.Errorf("got %q, want %q", result, "moderate")
	}
}

func TestParseScoringComplexity_GivenEmptyContent_ThenDefaultsToModerate(t *testing.T) {
	// Given: empty content
	result := parseScoringComplexity("")

	// Then: defaults to "moderate"
	if result != "moderate" {
		t.Errorf("got %q, want %q", result, "moderate")
	}
}

func TestParseScoringComplexity_GivenComplexityNotFirstLine_ThenStillParsed(t *testing.T) {
	// Given: complexity on the third line
	content := "name: task-01\ndescription: Some task\ncomplexity: simple\n"

	// When: we parse
	result := parseScoringComplexity(content)

	// Then: still finds "simple"
	if result != "simple" {
		t.Errorf("got %q, want %q", result, "simple")
	}
}

// --- splitLines ---

func TestSplitLines_GivenMultipleLines_ThenSplitCorrectly(t *testing.T) {
	// Given: a string with 3 lines
	input := "line1\nline2\nline3"

	// When: we split
	lines := splitLines(input)

	// Then: 3 lines returned
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("lines = %v", lines)
	}
}

func TestSplitLines_GivenTrailingNewline_ThenNoEmptyTrailingElement(t *testing.T) {
	// Given: a string ending with a newline
	input := "line1\nline2\n"

	// When: we split
	lines := splitLines(input)

	// Then: 2 lines (no empty trailing element)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
}

func TestSplitLines_GivenEmptyString_ThenEmptySlice(t *testing.T) {
	// Given: an empty string
	lines := splitLines("")

	// Then: empty slice (not nil, not single empty string)
	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0", len(lines))
	}
}

func TestSplitLines_GivenSingleLine_ThenOneElement(t *testing.T) {
	// Given: a string with no newlines
	lines := splitLines("hello")

	// Then: one element
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("got %v, want [hello]", lines)
	}
}

// --- parsePitfalls ---

func TestParsePitfalls_GivenValidJSON_ThenParsedCorrectly(t *testing.T) {
	// Given: valid JSON array of pitfall specs
	data := []PitfallSpec{
		{ID: "p1", Description: "Missing jitter", Pattern: `time\.Sleep`, AntiPattern: `jitter`},
		{ID: "p2", Description: "No idempotency"},
	}
	jsonData, _ := json.Marshal(data)

	// When: we parse
	result := parsePitfalls(jsonData)

	// Then: 2 pitfalls with correct fields
	if len(result) != 2 {
		t.Fatalf("got %d pitfalls, want 2", len(result))
	}
	if result[0].ID != "p1" {
		t.Errorf("first pitfall ID = %q, want p1", result[0].ID)
	}
	if result[0].Pattern != `time\.Sleep` {
		t.Errorf("first pitfall Pattern = %q", result[0].Pattern)
	}
}

func TestParsePitfalls_GivenInvalidJSON_ThenEmptySlice(t *testing.T) {
	// Given: invalid JSON
	data := []byte("not json at all")

	// When: we parse
	result := parsePitfalls(data)

	// Then: empty slice (graceful degradation)
	if len(result) != 0 {
		t.Errorf("got %d pitfalls, want 0 for invalid JSON", len(result))
	}
}

func TestParsePitfalls_GivenEmptyArray_ThenEmptySlice(t *testing.T) {
	// Given: empty JSON array
	result := parsePitfalls([]byte("[]"))

	// Then: empty slice
	if len(result) != 0 {
		t.Errorf("got %d pitfalls, want 0", len(result))
	}
}

// --- complexityTimeout ---

func TestComplexityTimeout_GivenSimple_ThenFiveMinutes(t *testing.T) {
	// Given/When: simple complexity
	result := complexityTimeout("simple")

	// Then: 5 minutes
	if result != 5*time.Minute {
		t.Errorf("got %v, want 5m", result)
	}
}

func TestComplexityTimeout_GivenModerate_ThenTenMinutes(t *testing.T) {
	// Given/When: moderate complexity
	result := complexityTimeout("moderate")

	// Then: 10 minutes
	if result != 10*time.Minute {
		t.Errorf("got %v, want 10m", result)
	}
}

func TestComplexityTimeout_GivenComplex_ThenTwentyMinutes(t *testing.T) {
	// Given/When: complex complexity
	result := complexityTimeout("complex")

	// Then: 20 minutes
	if result != 20*time.Minute {
		t.Errorf("got %v, want 20m", result)
	}
}

func TestComplexityTimeout_GivenUnknown_ThenDefaultsTenMinutes(t *testing.T) {
	// Given/When: unknown complexity value
	result := complexityTimeout("unknown")

	// Then: defaults to 10 minutes (moderate)
	if result != 10*time.Minute {
		t.Errorf("got %v, want 10m (default)", result)
	}
}

// --- copyDir ---

func TestCopyDir_GivenFlatDirectory_ThenAllFilesCopied(t *testing.T) {
	// Given: a source directory with 3 files
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "a.go"), []byte("package a"), 0644)
	os.WriteFile(filepath.Join(src, "b.go"), []byte("package b"), 0644)
	os.WriteFile(filepath.Join(src, "c.txt"), []byte("hello"), 0644)
	dst := t.TempDir()

	// When: we copy
	err := copyDir(src, dst)

	// Then: all 3 files exist in destination with correct content
	if err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if _, err := os.Stat(filepath.Join(dst, name)); os.IsNotExist(err) {
			t.Errorf("missing file: %s", name)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dst, "a.go"))
	if string(data) != "package a" {
		t.Errorf("a.go content = %q", string(data))
	}
}

func TestCopyDir_GivenNestedDirectories_ThenRecursivelyCopied(t *testing.T) {
	// Given: a source with nested subdirectories
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "sub", "deep"), 0755)
	os.WriteFile(filepath.Join(src, "root.go"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "mid.go"), []byte("mid"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "deep", "leaf.go"), []byte("leaf"), 0644)
	dst := t.TempDir()

	// When: we copy
	err := copyDir(src, dst)

	// Then: all nested files exist
	if err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	for _, path := range []string{"root.go", "sub/mid.go", "sub/deep/leaf.go"} {
		data, err := os.ReadFile(filepath.Join(dst, path))
		if err != nil {
			t.Errorf("missing nested file: %s", path)
			continue
		}
		if len(data) == 0 {
			t.Errorf("empty nested file: %s", path)
		}
	}
}

func TestCopyDir_GivenNonExistentSource_ThenErrorReturned(t *testing.T) {
	// Given: a source directory that doesn't exist
	dst := t.TempDir()

	// When: we try to copy
	err := copyDir("/nonexistent/path", dst)

	// Then: an error is returned
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

func TestCopyDir_GivenEmptyDirectory_ThenNoError(t *testing.T) {
	// Given: an empty source directory
	src := t.TempDir()
	dst := t.TempDir()

	// When: we copy
	err := copyDir(src, dst)

	// Then: no error (empty directory is valid)
	if err != nil {
		t.Fatalf("copyDir empty: %v", err)
	}
}

// --- discoverTasks ---

func TestDiscoverTasks_GivenTasksWithREADME_ThenDiscovered(t *testing.T) {
	// Given: a task corpus with 2 task directories containing README.md
	taskDir := t.TempDir()
	for _, taskID := range []string{"task-payment", "task-auth"} {
		dir := filepath.Join(taskDir, taskID)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Task: "+taskID), 0644)
	}

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, err := runner.discoverTasks()

	// Then: 2 tasks discovered
	if err != nil {
		t.Fatalf("discoverTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
}

func TestDiscoverTasks_GivenTaskWithoutREADME_ThenSkipped(t *testing.T) {
	// Given: a task directory without README.md
	taskDir := t.TempDir()
	os.MkdirAll(filepath.Join(taskDir, "task-broken"), 0755)
	os.WriteFile(filepath.Join(taskDir, "task-broken", "main.go"), []byte("package main"), 0644)
	// Also add a valid task
	os.MkdirAll(filepath.Join(taskDir, "task-valid"), 0755)
	os.WriteFile(filepath.Join(taskDir, "task-valid", "README.md"), []byte("# Valid"), 0644)

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, err := runner.discoverTasks()

	// Then: only the valid task is discovered
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("got %d tasks, want 1 (broken skipped)", len(tasks))
	}
	if tasks[0].id != "task-valid" {
		t.Errorf("task id = %q, want task-valid", tasks[0].id)
	}
}

func TestDiscoverTasks_GivenScoringYAML_ThenComplexityParsed(t *testing.T) {
	// Given: a task with scoring.yaml containing complexity
	taskDir := t.TempDir()
	dir := filepath.Join(taskDir, "task-scored")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Task"), 0644)
	os.WriteFile(filepath.Join(dir, "scoring.yaml"), []byte("complexity: complex\n"), 0644)

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, _ := runner.discoverTasks()

	// Then: complexity is "complex"
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks", len(tasks))
	}
	if tasks[0].complexity != "complex" {
		t.Errorf("complexity = %q, want complex", tasks[0].complexity)
	}
}

func TestDiscoverTasks_GivenPitfallsJSON_ThenPitfallsParsed(t *testing.T) {
	// Given: a task with pitfalls.yaml (JSON format) containing pitfall specs
	taskDir := t.TempDir()
	dir := filepath.Join(taskDir, "task-pitfalls")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Task"), 0644)

	pitfalls := []PitfallSpec{{ID: "p1", Description: "Missing jitter"}}
	pitData, _ := json.Marshal(pitfalls)
	os.WriteFile(filepath.Join(dir, "pitfalls.yaml"), pitData, 0644)

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, _ := runner.discoverTasks()

	// Then: pitfalls are parsed
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks", len(tasks))
	}
	if len(tasks[0].pitfalls) != 1 {
		t.Errorf("pitfalls = %d, want 1", len(tasks[0].pitfalls))
	}
	if tasks[0].pitfalls[0].ID != "p1" {
		t.Errorf("pitfall ID = %q, want p1", tasks[0].pitfalls[0].ID)
	}
}

func TestDiscoverTasks_GivenFilesInRoot_ThenSkippedNotDirectories(t *testing.T) {
	// Given: task corpus with a file at root level (not a directory)
	taskDir := t.TempDir()
	os.WriteFile(filepath.Join(taskDir, "notes.txt"), []byte("not a task"), 0644)
	os.MkdirAll(filepath.Join(taskDir, "task-real"), 0755)
	os.WriteFile(filepath.Join(taskDir, "task-real", "README.md"), []byte("# Real"), 0644)

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, _ := runner.discoverTasks()

	// Then: only directory task is discovered (file skipped)
	if len(tasks) != 1 {
		t.Errorf("got %d tasks, want 1", len(tasks))
	}
}

func TestDiscoverTasks_GivenEmptyCorpus_ThenEmptyList(t *testing.T) {
	// Given: an empty task corpus directory
	taskDir := t.TempDir()
	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, err := runner.discoverTasks()

	// Then: empty list, no error
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("got %d tasks, want 0", len(tasks))
	}
}

func TestDiscoverTasks_GivenTaskSpecContent_ThenReadFromREADME(t *testing.T) {
	// Given: a task with specific README content
	taskDir := t.TempDir()
	dir := filepath.Join(taskDir, "task-spec")
	os.MkdirAll(dir, 0755)
	expectedSpec := "# Payment Retry\n\nImplement retry logic with jitter."
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(expectedSpec), 0644)

	runner := &PipeRunner{taskDir: taskDir}

	// When: we discover tasks
	tasks, _ := runner.discoverTasks()

	// Then: spec contains the README content
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks", len(tasks))
	}
	if tasks[0].spec != expectedSpec {
		t.Errorf("spec = %q, want %q", tasks[0].spec, expectedSpec)
	}
}

// --- PipeRunner writeLog ---

func TestPipeRunner_GivenLogDir_WhenWriteLog_ThenLogFilesCreated(t *testing.T) {
	// Given: a runner with a log directory
	logDir := t.TempDir()
	runner := &PipeRunner{logDir: logDir}

	// When: we write a log with output and error
	runner.writeLog("test-run-001", "some output", nil)

	// Then: output.txt is created
	outPath := filepath.Join(logDir, "test-run-001", "output.txt")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output.txt: %v", err)
	}
	if string(data) != "some output" {
		t.Errorf("output = %q, want 'some output'", string(data))
	}
}

func TestPipeRunner_GivenEmptyLogDir_WhenWriteLog_ThenNoop(t *testing.T) {
	// Given: a runner with no log directory
	runner := &PipeRunner{logDir: ""}

	// When: we write a log (should not panic)
	runner.writeLog("test-run", "output", nil)

	// Then: no panic, no files created (nothing to verify except no error)
}

// --- NewPipeRunner ---

func TestNewPipeRunner_GivenParameters_ThenFieldsSet(t *testing.T) {
	// Given: constructor parameters
	runner := NewPipeRunner(nil, "/tasks", "/logs")

	// Then: fields are set
	if runner.taskDir != "/tasks" {
		t.Errorf("taskDir = %q, want /tasks", runner.taskDir)
	}
	if runner.logDir != "/logs" {
		t.Errorf("logDir = %q, want /logs", runner.logDir)
	}
}
