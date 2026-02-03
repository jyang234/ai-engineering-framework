package agents

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents an EDI skill with optional language filtering
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Languages   []string `yaml:"languages"` // nil or empty = all languages
	Content     string   `yaml:"-"`         // Markdown body
}

// LoadSkill loads and parses a skill file from ~/.claude/skills/{name}/SKILL.md
func LoadSkill(name string) (*Skill, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	path := filepath.Join(home, ".claude", "skills", name, "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill %s: %w", name, err)
	}

	skill, body, err := parseSkillFile(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill %s: %w", name, err)
	}

	skill.Content = body
	if skill.Name == "" {
		skill.Name = name
	}
	return skill, nil
}

// parseSkillFile parses YAML frontmatter and markdown body from a skill file
func parseSkillFile(content []byte) (*Skill, string, error) {
	reader := bufio.NewReader(bytes.NewReader(content))

	// Check for frontmatter delimiter
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(firstLine) != "---" {
		// No frontmatter, entire content is skill body
		return &Skill{}, string(content), nil
	}

	// Read frontmatter until closing ---
	var frontmatter strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, "", fmt.Errorf("unterminated frontmatter: %w", err)
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
	}

	// Parse YAML frontmatter
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &skill); err != nil {
		return nil, "", fmt.Errorf("invalid frontmatter: %w", err)
	}

	// Rest is markdown body
	var body strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		body.WriteString(line)
	}

	return &skill, strings.TrimSpace(body.String()), nil
}

// FilterSkills filters a list of skill names based on project languages.
// Skills with no language restriction (empty Languages field) are always included.
// Skills with language restrictions are included only if they match project languages.
// If projectLangs is empty, all skills are included (no filtering).
func FilterSkills(skillNames []string, projectLangs []string) []string {
	// No filtering if project has no language configuration
	if len(projectLangs) == 0 {
		return skillNames
	}

	var filtered []string
	for _, name := range skillNames {
		skill, err := LoadSkill(name)
		if err != nil {
			// On error loading skill, include it (fail open)
			filtered = append(filtered, name)
			continue
		}

		// No language restriction = always include
		if len(skill.Languages) == 0 {
			filtered = append(filtered, name)
			continue
		}

		// Check if skill's languages overlap with project languages
		if hasOverlap(skill.Languages, projectLangs) {
			filtered = append(filtered, name)
		}
	}

	return filtered
}

// hasOverlap returns true if the two slices share at least one element
func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	for _, s := range a {
		if set[s] {
			return true
		}
	}
	return false
}
