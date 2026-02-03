package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguages(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "Go project",
			files:    []string{"go.mod"},
			expected: []string{"go"},
		},
		{
			name:     "Python project with requirements",
			files:    []string{"requirements.txt"},
			expected: []string{"python"},
		},
		{
			name:     "Python project with pyproject",
			files:    []string{"pyproject.toml"},
			expected: []string{"python"},
		},
		{
			name:     "TypeScript project",
			files:    []string{"tsconfig.json"},
			expected: []string{"typescript"},
		},
		{
			name:     "JavaScript project",
			files:    []string{"package.json"},
			expected: []string{"javascript"},
		},
		{
			name:     "Rust project",
			files:    []string{"Cargo.toml"},
			expected: []string{"rust"},
		},
		{
			name:     "Polyglot Go and Python",
			files:    []string{"go.mod", "requirements.txt"},
			expected: []string{"go", "python"},
		},
		{
			name:     "No language markers",
			files:    []string{"README.md"},
			expected: []string{},
		},
		{
			name:     "Empty directory",
			files:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			dir := t.TempDir()

			// Create marker files
			for _, file := range tt.files {
				path := filepath.Join(dir, file)
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatalf("failed to create %s: %v", file, err)
				}
			}

			// Detect languages
			got := DetectLanguages(dir)

			// Check results (order-independent)
			if len(got) != len(tt.expected) {
				t.Errorf("DetectLanguages() = %v, want %v", got, tt.expected)
				return
			}

			gotMap := make(map[string]bool)
			for _, lang := range got {
				gotMap[lang] = true
			}

			for _, want := range tt.expected {
				if !gotMap[want] {
					t.Errorf("DetectLanguages() missing %s, got %v", want, got)
				}
			}
		})
	}
}

func TestDetectLanguages_NoDuplicates(t *testing.T) {
	dir := t.TempDir()

	// Create multiple Python markers
	for _, file := range []string{"requirements.txt", "pyproject.toml", "setup.py"} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte{}, 0644); err != nil {
			t.Fatalf("failed to create %s: %v", file, err)
		}
	}

	got := DetectLanguages(dir)

	// Should only have one "python" entry
	count := 0
	for _, lang := range got {
		if lang == "python" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 python entry, got %d in %v", count, got)
	}
}
