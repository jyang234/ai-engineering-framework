//go:build fts5

package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConditionBaseline(t *testing.T) {
	cond, err := NewCondition(ConditionBaseline, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cond.Name != ConditionBaseline {
		t.Errorf("name = %q, want %q", cond.Name, ConditionBaseline)
	}
	if cond.SystemPrompt != "" {
		t.Error("baseline should have empty system prompt")
	}
	if len(cond.Skills) != 0 {
		t.Error("baseline should have no skills")
	}
	if len(cond.Hooks) != 0 {
		t.Error("baseline should have no hooks")
	}
	if len(cond.RECALLSeeds) != 0 {
		t.Error("baseline should have no RECALL seeds")
	}
	if len(cond.AllowedTools) == 0 {
		t.Error("baseline should have allowed tools")
	}
}

func TestNewConditionAEFMinimal(t *testing.T) {
	skillDir := setupTestSkills(t)

	cond, err := NewCondition(ConditionAEFMinimal, skillDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cond.Name != ConditionAEFMinimal {
		t.Errorf("name = %q, want %q", cond.Name, ConditionAEFMinimal)
	}
	if cond.SystemPrompt == "" {
		t.Error("aef-minimal should have a system prompt")
	}
	if len(cond.Skills) != 4 {
		t.Errorf("skills = %d, want 4", len(cond.Skills))
	}
	if len(cond.Hooks) == 0 {
		t.Error("aef-minimal should have hooks")
	}
	if len(cond.RECALLSeeds) != 0 {
		t.Error("aef-minimal should have no RECALL seeds")
	}
	if len(cond.RECALLTools) != 0 {
		t.Error("aef-minimal should have no RECALL tools")
	}

	// Verify skill content is concatenated
	for _, skill := range aefSkills {
		if !strings.Contains(cond.SystemPrompt, "Skill: "+skill) {
			t.Errorf("system prompt missing skill: %s", skill)
		}
	}
}

func TestNewConditionAEFFull(t *testing.T) {
	skillDir := setupTestSkills(t)

	pitfalls := []PitfallSpec{
		{
			ID:          "pit-01",
			Description: "Missing jitter in retry",
			Seeds: []RECALLSeed{
				{Type: "failure", Title: "Thundering herd", Content: "Add jitter to backoff", Tags: []string{"retry"}},
			},
		},
		{
			ID:          "pit-02",
			Description: "No idempotency key",
			Seeds: []RECALLSeed{
				{Type: "failure", Title: "Duplicate payments", Content: "Use idempotency keys", Tags: []string{"payment"}},
			},
		},
	}

	cond, err := NewCondition(ConditionAEFFull, skillDir, pitfalls)
	if err != nil {
		t.Fatal(err)
	}
	if cond.Name != ConditionAEFFull {
		t.Errorf("name = %q, want %q", cond.Name, ConditionAEFFull)
	}
	if len(cond.RECALLSeeds) != 2 {
		t.Errorf("recall_seeds = %d, want 2", len(cond.RECALLSeeds))
	}
	if len(cond.RECALLTools) != 2 {
		t.Errorf("recall_tools = %d, want 2", len(cond.RECALLTools))
	}
	// aef-full should have base + recall tools
	if len(cond.AllowedTools) <= len(baseAllowedTools) {
		t.Error("aef-full should have more tools than baseline")
	}
}

func TestNewConditionUnknown(t *testing.T) {
	_, err := NewCondition("bogus", "", nil)
	if err == nil {
		t.Error("expected error for unknown condition")
	}
}

func TestParseConditionName(t *testing.T) {
	for _, name := range AllConditionNames() {
		got, err := ParseConditionName(string(name))
		if err != nil {
			t.Errorf("ParseConditionName(%q) error: %v", name, err)
		}
		if got != name {
			t.Errorf("ParseConditionName(%q) = %q, want %q", name, got, name)
		}
	}

	_, err := ParseConditionName("invalid")
	if err == nil {
		t.Error("expected error for invalid condition name")
	}
}

func TestWriteSystemPromptFile(t *testing.T) {
	skillDir := setupTestSkills(t)
	cond, err := NewCondition(ConditionAEFMinimal, skillDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	path, err := cond.WriteSystemPromptFile(tmpDir)
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

	// Baseline should return empty path
	base, _ := NewCondition(ConditionBaseline, "", nil)
	path, err = base.WriteSystemPromptFile(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("baseline path = %q, want empty", path)
	}
}

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
