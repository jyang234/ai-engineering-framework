package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConditionName identifies an evaluation condition.
type ConditionName string

const (
	ConditionBaseline   ConditionName = "baseline"
	ConditionAEFMinimal ConditionName = "aef-minimal"
	ConditionAEFFull    ConditionName = "aef-full"
)

// Condition holds the full configuration for an evaluation condition.
type Condition struct {
	Name         ConditionName `json:"name"`
	SystemPrompt string        `json:"system_prompt"` // Combined skill content for --append-system-prompt-file
	Skills       []string      `json:"skills"`        // Skill names loaded
	Hooks        []HookConfig  `json:"hooks"`         // Hook configurations
	RECALLSeeds  []RECALLSeed  `json:"recall_seeds"`  // Items to pre-seed in RECALL
	RECALLTools  []string      `json:"recall_tools"`  // RECALL tool names available to agent
	AllowedTools []string      `json:"allowed_tools"` // Tools the agent can use
}

// HookConfig represents a single hook in Claude Code settings.
type HookConfig struct {
	Matcher string `json:"matcher"` // Event matcher (e.g., "PostToolUse", "PreToolUse")
	Command string `json:"command"` // Shell command to execute
}

// RECALLSeed represents an item to pre-seed in RECALL before a run.
type RECALLSeed struct {
	Type    string   `json:"type" yaml:"type"`
	Title   string   `json:"title" yaml:"title"`
	Content string   `json:"content" yaml:"content"`
	Tags    []string `json:"tags" yaml:"tags"`
}

// DetectionMethod specifies how a pitfall is detected during scoring.
type DetectionMethod string

const (
	// DetectionGrep checks for regex patterns in source files (default).
	DetectionGrep DetectionMethod = "grep"
	// DetectionTest checks if a specific test name passes.
	DetectionTest DetectionMethod = "test"
	// DetectionJudge asks an LLM judge whether the pitfall was avoided.
	DetectionJudge DetectionMethod = "judge"
)

// PitfallDetection configures how a pitfall is detected.
type PitfallDetection struct {
	Method  DetectionMethod `json:"method" yaml:"method"`   // grep, test, or judge
	Pattern string          `json:"pattern" yaml:"pattern"` // regex (grep), test name (test), or query (judge)
	Files   []string        `json:"files" yaml:"files"`     // file glob patterns to check (grep only)
}

// PitfallSpec defines a known pitfall for a task, loaded from pitfalls.yaml.
type PitfallSpec struct {
	ID          string           `json:"id" yaml:"id"`
	Type        string           `json:"type,omitempty" yaml:"type,omitempty"`
	Title       string           `json:"title,omitempty" yaml:"title,omitempty"`
	Description string           `json:"description" yaml:"description"`
	Content     string           `json:"content,omitempty" yaml:"content,omitempty"`
	Tags        []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
	Pattern     string           `json:"pattern" yaml:"pattern"`           // Regex to detect if pitfall was hit
	AntiPattern string           `json:"anti_pattern" yaml:"anti_pattern"` // Regex for code that avoids the pitfall
	Detection   PitfallDetection `json:"detection" yaml:"detection"`       // Structured detection config
	Seeds       []RECALLSeed     `json:"seeds" yaml:"seeds"`               // RECALL items that would help avoid this pitfall
}

// baseAllowedTools is the tool set shared across all conditions.
var baseAllowedTools = []string{
	"Edit", "Write", "Read", "Glob", "Grep",
	"Bash(go build:*)", "Bash(go test:*)", "Bash(go vet:*)",
	"Bash(gofumpt:*)", "Bash(golangci-lint:*)", "Bash(go mod tidy:*)",
}

// aefSkills is the skill set used by aef-minimal and aef-full.
var aefSkills = []string{
	"edi-core", "coding", "testing", "plan-review",
}

// aefHooks is the hook set used by aef-minimal and aef-full.
var aefHooks = []HookConfig{
	{Matcher: "PostToolUse:Write:*.go", Command: "gofumpt -w $FILE"},
	{Matcher: "PostToolUse:Edit:*.go", Command: "gofumpt -w $FILE"},
}

// recallTools is the tool set added for aef-full.
var recallTools = []string{
	"recall_search", "recall_add",
}

// NewCondition creates a Condition for the given name and optional skill directory.
// skillDir should point to the skills directory (e.g., edi/internal/assets/skills/).
// For baseline, skillDir can be empty.
func NewCondition(name ConditionName, skillDir string, pitfalls []PitfallSpec) (*Condition, error) {
	switch name {
	case ConditionBaseline:
		return newBaseline(), nil
	case ConditionAEFMinimal:
		return newAEFMinimal(skillDir)
	case ConditionAEFFull:
		return newAEFFull(skillDir, pitfalls)
	default:
		return nil, fmt.Errorf("unknown condition: %q", name)
	}
}

func newBaseline() *Condition {
	return &Condition{
		Name:         ConditionBaseline,
		AllowedTools: baseAllowedTools,
	}
}

// recallUnavailablePreamble is prepended to AEF-minimal system prompts to prevent
// the model from attempting to use RECALL tools referenced in skill files.
// This resolves spec Gap 1: skills reference recall_search and flight_recorder_log
// but these tools are not available in the AEF-minimal condition.
const recallUnavailablePreamble = `IMPORTANT: RECALL tools (recall_search, recall_get, recall_add, recall_feedback, flight_recorder_log) are NOT available in this session. Any instructions in the skills below that reference these tools should be skipped. Focus on using the file and code tools provided.

---

`

func newAEFMinimal(skillDir string) (*Condition, error) {
	prompt, err := loadSkills(skillDir, aefSkills)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	return &Condition{
		Name:         ConditionAEFMinimal,
		SystemPrompt: recallUnavailablePreamble + prompt,
		Skills:       aefSkills,
		Hooks:        aefHooks,
		AllowedTools: baseAllowedTools,
	}, nil
}

func newAEFFull(skillDir string, pitfalls []PitfallSpec) (*Condition, error) {
	prompt, err := loadSkills(skillDir, aefSkills)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}

	var seeds []RECALLSeed
	for _, p := range pitfalls {
		seeds = append(seeds, p.Seeds...)
	}

	tools := append(baseAllowedTools, recallTools...)

	return &Condition{
		Name:         ConditionAEFFull,
		SystemPrompt: prompt,
		Skills:       aefSkills,
		Hooks:        aefHooks,
		RECALLSeeds:  seeds,
		RECALLTools:  recallTools,
		AllowedTools: tools,
	}, nil
}

// loadSkills reads and concatenates skill SKILL.md files from the given directory.
func loadSkills(skillDir string, skills []string) (string, error) {
	if skillDir == "" {
		return "", fmt.Errorf("skillDir is required for non-baseline conditions")
	}

	var parts []string
	for _, skill := range skills {
		path := filepath.Join(skillDir, skill, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read skill %s: %w", skill, err)
		}
		parts = append(parts, fmt.Sprintf("# Skill: %s\n\n%s", skill, strings.TrimSpace(string(data))))
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// WriteSystemPromptFile writes the condition's system prompt to a temp file
// suitable for use with --append-system-prompt-file.
func (c *Condition) WriteSystemPromptFile(dir string) (string, error) {
	if c.SystemPrompt == "" {
		return "", nil
	}
	path := filepath.Join(dir, "system-prompt.md")
	if err := os.WriteFile(path, []byte(c.SystemPrompt), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// AllConditionNames returns all valid condition names.
func AllConditionNames() []ConditionName {
	return []ConditionName{ConditionBaseline, ConditionAEFMinimal, ConditionAEFFull}
}

// ParseConditionName parses a string into a ConditionName.
func ParseConditionName(s string) (ConditionName, error) {
	switch ConditionName(s) {
	case ConditionBaseline, ConditionAEFMinimal, ConditionAEFFull:
		return ConditionName(s), nil
	default:
		return "", fmt.Errorf("invalid condition: %q (valid: baseline, aef-minimal, aef-full)", s)
	}
}
