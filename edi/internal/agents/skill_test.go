package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantName    string
		wantLangs   []string
		wantContent string
	}{
		{
			name: "skill with languages",
			content: `---
name: golang-idioms
description: Go idioms
languages: [go]
---

# Go Idioms

Content here.`,
			wantName:    "golang-idioms",
			wantLangs:   []string{"go"},
			wantContent: "# Go Idioms\n\nContent here.",
		},
		{
			name: "skill without languages",
			content: `---
name: coding
description: Coding standards
---

# Coding

General coding content.`,
			wantName:    "coding",
			wantLangs:   nil,
			wantContent: "# Coding\n\nGeneral coding content.",
		},
		{
			name: "skill with multiple languages",
			content: `---
name: web-idioms
description: Web development idioms
languages: [javascript, typescript]
---

# Web Idioms`,
			wantName:    "web-idioms",
			wantLangs:   []string{"javascript", "typescript"},
			wantContent: "# Web Idioms",
		},
		{
			name:        "no frontmatter",
			content:     "# Just Content\n\nNo YAML frontmatter.",
			wantName:    "",
			wantLangs:   nil,
			wantContent: "# Just Content\n\nNo YAML frontmatter.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, content, err := parseSkillFile([]byte(tt.content))
			if err != nil {
				t.Fatalf("parseSkillFile() error = %v", err)
			}

			if skill.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", skill.Name, tt.wantName)
			}

			if len(skill.Languages) != len(tt.wantLangs) {
				t.Errorf("Languages = %v, want %v", skill.Languages, tt.wantLangs)
			} else {
				for i, lang := range skill.Languages {
					if lang != tt.wantLangs[i] {
						t.Errorf("Languages[%d] = %q, want %q", i, lang, tt.wantLangs[i])
					}
				}
			}

			if content != tt.wantContent {
				t.Errorf("Content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

func TestFilterSkills(t *testing.T) {
	// Setup: create temp skills directory
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home directory")
	}

	skillsDir := filepath.Join(home, ".claude", "skills")

	// Create test skills
	testSkills := map[string]string{
		"general": `---
name: general
description: General skill
---
Content`,
		"go-only": `---
name: go-only
description: Go only skill
languages: [go]
---
Content`,
		"python-only": `---
name: python-only
description: Python only skill
languages: [python]
---
Content`,
		"multi-lang": `---
name: multi-lang
description: Multi-language skill
languages: [go, python]
---
Content`,
	}

	// Create skill files
	for name, content := range testSkills {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write skill: %v", err)
		}
	}

	// Cleanup after test
	defer func() {
		for name := range testSkills {
			os.RemoveAll(filepath.Join(skillsDir, name))
		}
	}()

	tests := []struct {
		name         string
		skills       []string
		projectLangs []string
		want         []string
	}{
		{
			name:         "no project languages - include all",
			skills:       []string{"general", "go-only", "python-only"},
			projectLangs: []string{},
			want:         []string{"general", "go-only", "python-only"},
		},
		{
			name:         "go project - filter python",
			skills:       []string{"general", "go-only", "python-only"},
			projectLangs: []string{"go"},
			want:         []string{"general", "go-only"},
		},
		{
			name:         "python project - filter go",
			skills:       []string{"general", "go-only", "python-only"},
			projectLangs: []string{"python"},
			want:         []string{"general", "python-only"},
		},
		{
			name:         "polyglot project - include both",
			skills:       []string{"general", "go-only", "python-only", "multi-lang"},
			projectLangs: []string{"go", "python"},
			want:         []string{"general", "go-only", "python-only", "multi-lang"},
		},
		{
			name:         "multi-lang skill matches go",
			skills:       []string{"multi-lang"},
			projectLangs: []string{"go"},
			want:         []string{"multi-lang"},
		},
		{
			name:         "unknown skill - fail open",
			skills:       []string{"nonexistent"},
			projectLangs: []string{"go"},
			want:         []string{"nonexistent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSkills(tt.skills, tt.projectLangs)

			if len(got) != len(tt.want) {
				t.Errorf("FilterSkills() = %v, want %v", got, tt.want)
				return
			}

			for i, skill := range got {
				if skill != tt.want[i] {
					t.Errorf("FilterSkills()[%d] = %q, want %q", i, skill, tt.want[i])
				}
			}
		})
	}
}

func TestHasOverlap(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"both empty", []string{}, []string{}, false},
		{"a empty", []string{}, []string{"go"}, false},
		{"b empty", []string{"go"}, []string{}, false},
		{"exact match", []string{"go"}, []string{"go"}, true},
		{"partial overlap", []string{"go", "rust"}, []string{"go", "python"}, true},
		{"no overlap", []string{"go", "rust"}, []string{"python", "java"}, false},
		{"multiple overlaps", []string{"go", "python"}, []string{"go", "python"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasOverlap(tt.a, tt.b); got != tt.want {
				t.Errorf("hasOverlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
