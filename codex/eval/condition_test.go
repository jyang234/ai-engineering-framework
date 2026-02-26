//go:build fts5

package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	dividerCount := strings.Count(cond.SystemPrompt, "\n\n---\n\n")
	expectedDividers := len(aefSkills) - 1 // N skills = N-1 dividers
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
