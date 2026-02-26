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
// Condition Configurator Tests — Given-When-Then
// =============================================================================

// --- Baseline Condition ---

func TestCondition_GivenBaseline_WhenCreated_ThenNoSkillsNoHooksNoRECALL(t *testing.T) {
	// Given: we request the baseline condition
	// When: we create it
	cond, err := NewCondition(ConditionBaseline, "", nil)

	// Then: it should have no system prompt, no skills, no hooks, no RECALL
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cond.Name != ConditionBaseline {
		t.Errorf("name = %q, want %q", cond.Name, ConditionBaseline)
	}
	if cond.SystemPrompt != "" {
		t.Error("baseline should have empty system prompt")
	}
	if len(cond.Skills) != 0 {
		t.Errorf("baseline skills = %d, want 0", len(cond.Skills))
	}
	if len(cond.Hooks) != 0 {
		t.Errorf("baseline hooks = %d, want 0", len(cond.Hooks))
	}
	if len(cond.RECALLSeeds) != 0 {
		t.Errorf("baseline RECALL seeds = %d, want 0", len(cond.RECALLSeeds))
	}
	if len(cond.RECALLTools) != 0 {
		t.Errorf("baseline RECALL tools = %d, want 0", len(cond.RECALLTools))
	}
}

func TestCondition_GivenBaseline_WhenCreated_ThenBaseToolsAvailable(t *testing.T) {
	// Given/When: a baseline condition
	cond, _ := NewCondition(ConditionBaseline, "", nil)

	// Then: it should have the standard set of allowed tools (Edit, Write, Read, etc.)
	if len(cond.AllowedTools) == 0 {
		t.Fatal("baseline should have allowed tools")
	}
	toolSet := make(map[string]bool)
	for _, tool := range cond.AllowedTools {
		toolSet[tool] = true
	}
	for _, expected := range []string{"Edit", "Write", "Read", "Glob", "Grep"} {
		if !toolSet[expected] {
			t.Errorf("baseline missing tool %q", expected)
		}
	}
}

// --- AEF-Minimal Condition ---

func TestCondition_GivenAEFMinimal_WhenCreated_ThenSkillsLoadedAndConcatenated(t *testing.T) {
	// Given: a skill directory with all required skill files
	skillDir := setupTestSkills(t)

	// When: we create the aef-minimal condition
	cond, err := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: 4 skills should be loaded and concatenated into the system prompt
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cond.Skills) != 4 {
		t.Errorf("skills count = %d, want 4", len(cond.Skills))
	}
	for _, skill := range []string{"edi-core", "coding", "testing", "plan-review"} {
		if !strings.Contains(cond.SystemPrompt, "# Skill: "+skill) {
			t.Errorf("system prompt missing skill header: %s", skill)
		}
		if !strings.Contains(cond.SystemPrompt, "Test content for "+skill+" skill.") {
			t.Errorf("system prompt missing skill content for: %s", skill)
		}
	}
}

func TestCondition_GivenAEFMinimal_WhenCreated_ThenHooksConfigured(t *testing.T) {
	// Given: a skill directory
	skillDir := setupTestSkills(t)

	// When: we create the aef-minimal condition
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: hooks should be configured for gofumpt on Write/Edit of .go files
	if len(cond.Hooks) == 0 {
		t.Fatal("aef-minimal should have hooks")
	}
	foundWriteHook := false
	for _, h := range cond.Hooks {
		if strings.Contains(h.Matcher, "Write") && strings.Contains(h.Command, "gofumpt") {
			foundWriteHook = true
		}
	}
	if !foundWriteHook {
		t.Error("missing gofumpt hook for Write operations")
	}
}

func TestCondition_GivenAEFMinimal_WhenCreated_ThenNoRECALL(t *testing.T) {
	// Given: a skill directory
	skillDir := setupTestSkills(t)

	// When: we create the aef-minimal condition
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: no RECALL seeds or tools should be present (that's aef-full only)
	if len(cond.RECALLSeeds) != 0 {
		t.Errorf("aef-minimal RECALL seeds = %d, want 0", len(cond.RECALLSeeds))
	}
	if len(cond.RECALLTools) != 0 {
		t.Errorf("aef-minimal RECALL tools = %d, want 0", len(cond.RECALLTools))
	}
}

// --- AEF-Full Condition ---

func TestCondition_GivenAEFFull_WhenCreatedWithPitfalls_ThenRECALLSeedsFromPitfalls(t *testing.T) {
	// Given: a skill directory and 3 pitfalls with 1 seed each
	skillDir := setupTestSkills(t)
	pitfalls := []PitfallSpec{
		{ID: "p1", Seeds: []RECALLSeed{{Type: "failure", Title: "Seed A", Content: "content a"}}},
		{ID: "p2", Seeds: []RECALLSeed{{Type: "failure", Title: "Seed B", Content: "content b"}}},
		{ID: "p3", Seeds: []RECALLSeed{{Type: "pattern", Title: "Seed C", Content: "content c"}}},
	}

	// When: we create the aef-full condition
	cond, err := NewCondition(ConditionAEFFull, skillDir, pitfalls)

	// Then: 3 RECALL seeds should be present (one per pitfall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cond.RECALLSeeds) != 3 {
		t.Errorf("RECALL seeds = %d, want 3", len(cond.RECALLSeeds))
	}
	// And RECALL tools (recall_search, recall_add) should be in the tool list
	if len(cond.RECALLTools) != 2 {
		t.Errorf("RECALL tools = %d, want 2", len(cond.RECALLTools))
	}
}

func TestCondition_GivenAEFFull_WhenCreated_ThenToolsIncludeBaseAndRECALL(t *testing.T) {
	// Given: aef-full condition
	skillDir := setupTestSkills(t)
	cond, _ := NewCondition(ConditionAEFFull, skillDir, nil)

	// Then: AllowedTools should be a superset of baseAllowedTools
	if len(cond.AllowedTools) <= len(baseAllowedTools) {
		t.Error("aef-full should have more tools than baseline (base + recall)")
	}
	// Should contain recall_search
	found := false
	for _, t2 := range cond.AllowedTools {
		if t2 == "recall_search" {
			found = true
		}
	}
	if !found {
		t.Error("aef-full AllowedTools should include recall_search")
	}
}

func TestCondition_GivenAEFFullWithNoPitfalls_WhenCreated_ThenEmptySeedsButRECALLToolsPresent(t *testing.T) {
	// Given: aef-full with no pitfalls
	skillDir := setupTestSkills(t)
	cond, err := NewCondition(ConditionAEFFull, skillDir, nil)

	// Then: seeds empty, but RECALL tools still available
	if err != nil {
		t.Fatal(err)
	}
	if len(cond.RECALLSeeds) != 0 {
		t.Errorf("seeds = %d, want 0 (no pitfalls)", len(cond.RECALLSeeds))
	}
	if len(cond.RECALLTools) != 2 {
		t.Errorf("RECALL tools = %d, want 2", len(cond.RECALLTools))
	}
}

// --- Error Cases ---

func TestCondition_GivenUnknownName_WhenCreated_ThenErrorReturned(t *testing.T) {
	// Given: an invalid condition name
	// When: we try to create it
	_, err := NewCondition("bogus-condition", "", nil)

	// Then: an error with the invalid name is returned
	if err == nil {
		t.Fatal("expected error for unknown condition name")
	}
	if !strings.Contains(err.Error(), "bogus-condition") {
		t.Errorf("error should mention the invalid name, got: %v", err)
	}
}

func TestCondition_GivenAEFMinimal_WhenSkillDirEmpty_ThenErrorReturned(t *testing.T) {
	// Given: aef-minimal requires a skillDir but we pass empty string
	// When: we try to create it
	_, err := NewCondition(ConditionAEFMinimal, "", nil)

	// Then: an error about missing skillDir is returned
	if err == nil {
		t.Fatal("expected error when skillDir is empty for aef-minimal")
	}
}

func TestCondition_GivenAEFMinimal_WhenSkillFileMissing_ThenErrorReturned(t *testing.T) {
	// Given: a skill directory that's missing one of the required skill files
	dir := t.TempDir()
	// Only create 3 of the 4 required skills
	for _, skill := range []string{"edi-core", "coding", "testing"} {
		os.MkdirAll(filepath.Join(dir, skill), 0755)
		os.WriteFile(filepath.Join(dir, skill, "SKILL.md"), []byte("stub"), 0644)
	}
	// "plan-review" is missing

	// When: we try to create the condition
	_, err := NewCondition(ConditionAEFMinimal, dir, nil)

	// Then: an error about the missing skill is returned
	if err == nil {
		t.Fatal("expected error for missing plan-review skill")
	}
}

// --- ParseConditionName ---

func TestParseConditionName_GivenValidNames_ThenParsedCorrectly(t *testing.T) {
	for _, name := range []ConditionName{ConditionBaseline, ConditionAEFMinimal, ConditionAEFFull} {
		t.Run(string(name), func(t *testing.T) {
			// Given: a valid condition name string
			// When: we parse it
			got, err := ParseConditionName(string(name))

			// Then: no error and correct ConditionName returned
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != name {
				t.Errorf("got %q, want %q", got, name)
			}
		})
	}
}

func TestParseConditionName_GivenInvalidName_ThenErrorReturned(t *testing.T) {
	// Given: an invalid condition name string
	// When: we parse it
	_, err := ParseConditionName("super-mode")

	// Then: an error listing valid options is returned
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "baseline") {
		t.Error("error should list valid condition names")
	}
}

// --- WriteSystemPromptFile ---

func TestCondition_GivenAEFMinimal_WhenSystemPromptFileWritten_ThenFileContainsFullPrompt(t *testing.T) {
	// Given: an aef-minimal condition with a non-empty system prompt
	skillDir := setupTestSkills(t)
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)
	tmpDir := t.TempDir()

	// When: we write the system prompt file
	path, err := cond.WriteSystemPromptFile(tmpDir)

	// Then: the file is created with the exact system prompt content
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != cond.SystemPrompt {
		t.Error("file content doesn't match system prompt")
	}
}

func TestCondition_GivenBaseline_WhenSystemPromptFileWritten_ThenEmptyPathReturned(t *testing.T) {
	// Given: a baseline condition (no system prompt)
	cond, _ := NewCondition(ConditionBaseline, "", nil)

	// When: we try to write the system prompt file
	path, err := cond.WriteSystemPromptFile(t.TempDir())

	// Then: empty path is returned (no file needed)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path for baseline, got %q", path)
	}
}

// --- AllConditionNames ---

func TestAllConditionNames_ReturnsAllThreeConditions(t *testing.T) {
	// Given/When: we request all condition names
	names := AllConditionNames()

	// Then: exactly 3 names are returned
	if len(names) != 3 {
		t.Fatalf("expected 3 condition names, got %d", len(names))
	}

	nameSet := make(map[ConditionName]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []ConditionName{ConditionBaseline, ConditionAEFMinimal, ConditionAEFFull} {
		if !nameSet[expected] {
			t.Errorf("missing condition name: %s", expected)
		}
	}
}

// --- Skill Content Separation ---

func TestCondition_GivenMultipleSkills_WhenLoaded_ThenSeparatedByDividers(t *testing.T) {
	// Given: a skill directory with multiple skills
	skillDir := setupTestSkills(t)

	// When: we create the condition
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: skills are separated by "---" dividers
	// AEF-minimal includes a RECALL-unavailable preamble ending with a divider,
	// so total dividers = (N-1 between skills) + 1 from preamble
	dividerCount := strings.Count(cond.SystemPrompt, "\n\n---\n\n")
	expectedDividers := len(aefSkills) - 1 + 1 // N-1 between skills + 1 preamble separator
	if dividerCount != expectedDividers {
		t.Errorf("divider count = %d, want %d", dividerCount, expectedDividers)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// setupTestSkills creates a temporary skill directory with stub SKILL.md files.
func setupTestSkills(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, skill := range aefSkills {
		skillDir := filepath.Join(dir, skill)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := "Test content for " + skill + " skill.\n\nThis is a stub."
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// =============================================================================
// RECALL-Unavailable Preamble Tests — Given-When-Then (spec Gap 1)
// =============================================================================

func TestCondition_GivenAEFMinimal_WhenCreated_ThenSystemPromptContainsRECALLPreamble(t *testing.T) {
	// Given: an aef-minimal condition
	skillDir := setupTestSkills(t)

	// When: we create it
	cond, err := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: the system prompt starts with the RECALL-unavailable preamble
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cond.SystemPrompt, "RECALL tools") {
		t.Error("aef-minimal system prompt should contain RECALL-unavailable preamble")
	}
	if !strings.Contains(cond.SystemPrompt, "NOT available") {
		t.Error("preamble should mention RECALL tools are NOT available")
	}
}

func TestCondition_GivenAEFMinimal_WhenCreated_ThenPreambleMentionsAllFiveRECALLTools(t *testing.T) {
	// Given: an aef-minimal condition
	skillDir := setupTestSkills(t)
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: the preamble lists all 5 RECALL tools by name
	for _, tool := range []string{"recall_search", "recall_get", "recall_add", "recall_feedback", "flight_recorder_log"} {
		if !strings.Contains(cond.SystemPrompt, tool) {
			t.Errorf("preamble should mention %q", tool)
		}
	}
}

func TestCondition_GivenAEFMinimal_WhenCreated_ThenPreamblePrecedesSkillContent(t *testing.T) {
	// Given: an aef-minimal condition
	skillDir := setupTestSkills(t)
	cond, _ := NewCondition(ConditionAEFMinimal, skillDir, nil)

	// Then: the preamble comes before any skill content
	preambleIdx := strings.Index(cond.SystemPrompt, "RECALL tools")
	skillIdx := strings.Index(cond.SystemPrompt, "# Skill:")
	if preambleIdx < 0 || skillIdx < 0 {
		t.Fatal("both preamble and skills should be present")
	}
	if preambleIdx > skillIdx {
		t.Error("preamble should precede skill content")
	}
}

func TestCondition_GivenAEFFull_WhenCreated_ThenNoRECALLPreamble(t *testing.T) {
	// Given: an aef-full condition (RECALL IS available)
	skillDir := setupTestSkills(t)

	// When: we create it
	cond, _ := NewCondition(ConditionAEFFull, skillDir, nil)

	// Then: no RECALL-unavailable preamble
	if strings.Contains(cond.SystemPrompt, "NOT available") {
		t.Error("aef-full should NOT have RECALL-unavailable preamble")
	}
}

func TestCondition_GivenBaseline_WhenCreated_ThenNoRECALLPreamble(t *testing.T) {
	// Given: a baseline condition
	cond, _ := NewCondition(ConditionBaseline, "", nil)

	// Then: no preamble (empty system prompt)
	if cond.SystemPrompt != "" {
		t.Error("baseline should have empty system prompt")
	}
}

// =============================================================================
// Detection Types Tests — Given-When-Then
// =============================================================================

func TestDetectionMethod_Constants_MatchSpecValues(t *testing.T) {
	// Given: the detection method constants
	// Then: they match the spec's three methods
	if DetectionGrep != "grep" {
		t.Errorf("DetectionGrep = %q, want grep", DetectionGrep)
	}
	if DetectionTest != "test" {
		t.Errorf("DetectionTest = %q, want test", DetectionTest)
	}
	if DetectionJudge != "judge" {
		t.Errorf("DetectionJudge = %q, want judge", DetectionJudge)
	}
}

func TestPitfallSpec_GivenYAMLTags_ThenAllFieldsHaveYAMLAnnotations(t *testing.T) {
	// Given: a PitfallSpec with all fields populated
	// This test verifies the struct has yaml tags by round-tripping through YAML
	yaml := `
- id: test-yaml-tags
  type: failure
  title: "Test Title"
  description: "Test Description"
  content: "Test Content"
  tags: ["a", "b"]
  pattern: "bad_pattern"
  anti_pattern: "good_pattern"
  detection:
    method: grep
    pattern: "detect"
    files: ["*.go"]
  seeds:
    - type: failure
      title: "Seed"
      content: "Seed content"
      tags: ["x"]
`
	pitfalls, err := ParsePitfallsYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}
	if len(pitfalls) != 1 {
		t.Fatalf("got %d pitfalls, want 1", len(pitfalls))
	}

	p := pitfalls[0]
	// Then: all fields round-trip correctly
	if p.ID != "test-yaml-tags" {
		t.Errorf("ID = %q", p.ID)
	}
	if p.Type != "failure" {
		t.Errorf("Type = %q", p.Type)
	}
	if p.Title != "Test Title" {
		t.Errorf("Title = %q", p.Title)
	}
	if p.Content != "Test Content" {
		t.Errorf("Content = %q", p.Content)
	}
	if len(p.Tags) != 2 {
		t.Errorf("Tags = %d, want 2", len(p.Tags))
	}
	if p.Pattern != "bad_pattern" {
		t.Errorf("Pattern = %q", p.Pattern)
	}
	if p.AntiPattern != "good_pattern" {
		t.Errorf("AntiPattern = %q", p.AntiPattern)
	}
	if p.Detection.Method != DetectionGrep {
		t.Errorf("Detection.Method = %q", p.Detection.Method)
	}
	if len(p.Seeds) != 1 {
		t.Errorf("Seeds = %d, want 1", len(p.Seeds))
	}
}

// =============================================================================
// ResultsDB New Methods Tests — Given-When-Then
// =============================================================================

func TestResultsDB_GivenPath_WhenOpened_ThenPathAccessible(t *testing.T) {
	// Given: a results database opened at a specific path
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-path.db")
	db, err := OpenResultsDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Then: Path() returns the original path
	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}
}

func TestResultsDB_GivenMultipleExperiments_WhenListExperiments_ThenAllListed(t *testing.T) {
	// Given: runs across 3 experiments
	db := openTestDB(t)
	defer db.Close()

	for _, exp := range []string{"3A", "3A", "3B", "3C", "3C", "3C"} {
		run := &EvalRun{
			RunID:      exp + "-list-" + time.Now().Format("150405.000000000"),
			Experiment: exp, Condition: "baseline", TaskID: "task-01",
			Attempt: 1, StartedAt: time.Now().UTC(),
		}
		if err := db.InsertRun(run); err != nil {
			t.Fatal(err)
		}
	}

	// When: we list experiments
	stats, err := db.ListExperiments()

	// Then: 3 experiments with correct run counts
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 3 {
		t.Fatalf("got %d experiments, want 3", len(stats))
	}

	statMap := make(map[string]int)
	for _, s := range stats {
		statMap[s.Experiment] = s.RunCount
	}
	if statMap["3A"] != 2 {
		t.Errorf("3A runs = %d, want 2", statMap["3A"])
	}
	if statMap["3C"] != 3 {
		t.Errorf("3C runs = %d, want 3", statMap["3C"])
	}
}

func TestResultsDB_GivenEmptyDB_WhenListExperiments_ThenEmptyList(t *testing.T) {
	// Given: an empty database
	db := openTestDB(t)
	defer db.Close()

	// When: we list experiments
	stats, err := db.ListExperiments()

	// Then: empty list
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Errorf("got %d experiments, want 0", len(stats))
	}
}

// =============================================================================
// TaskInfo Tests — Given-When-Then
// =============================================================================

func TestTaskInfo_GivenExportedType_ThenFieldsAccessible(t *testing.T) {
	// Given: a TaskInfo with all fields set
	info := TaskInfo{
		ID:         "task-payment",
		Complexity: "moderate",
		Spec:       "# Payment Retry\nImplement retry logic.",
		Pitfalls:   []PitfallSpec{{ID: "p1", Description: "Missing jitter"}},
	}

	// Then: all fields are accessible
	if info.ID != "task-payment" {
		t.Errorf("ID = %q", info.ID)
	}
	if info.Complexity != "moderate" {
		t.Errorf("Complexity = %q", info.Complexity)
	}
	if info.Spec != "# Payment Retry\nImplement retry logic." {
		t.Errorf("Spec = %q", info.Spec)
	}
	if len(info.Pitfalls) != 1 {
		t.Errorf("Pitfalls = %d", len(info.Pitfalls))
	}
}

func TestPipeRunner_DiscoverTasks_GivenValidCorpus_ThenExportedTasksReturned(t *testing.T) {
	// Given: a task corpus with 2 tasks
	taskDir := t.TempDir()
	for _, id := range []string{"task-a", "task-b"} {
		dir := filepath.Join(taskDir, id)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+id), 0644)
	}
	runner := NewPipeRunner(nil, taskDir, "")

	// When: we discover tasks (exported method)
	tasks, err := runner.DiscoverTasks()

	// Then: 2 TaskInfo items returned
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].Spec == "" {
		t.Error("task spec should not be empty")
	}
}
