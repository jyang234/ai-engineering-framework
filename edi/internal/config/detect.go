package config

import (
	"os"
	"path/filepath"
)

// languageMarkers maps files/directories to their associated language
var languageMarkers = map[string]string{
	// Go
	"go.mod": "go",
	"go.sum": "go",

	// Python
	"requirements.txt": "python",
	"pyproject.toml":   "python",
	"setup.py":         "python",
	"Pipfile":          "python",

	// JavaScript/TypeScript
	"package.json":   "javascript",
	"tsconfig.json":  "typescript",
	"jsconfig.json":  "javascript",
	"deno.json":      "typescript",

	// Rust
	"Cargo.toml": "rust",

	// Ruby
	"Gemfile": "ruby",

	// Java/Kotlin
	"pom.xml":         "java",
	"build.gradle":    "java",
	"build.gradle.kts": "kotlin",

	// C#/.NET
	"*.csproj": "csharp",
	"*.sln":    "csharp",

	// PHP
	"composer.json": "php",

	// Elixir
	"mix.exs": "elixir",

	// Scala
	"build.sbt": "scala",
}

// DetectLanguages scans a project root for language markers
func DetectLanguages(root string) []string {
	seen := make(map[string]bool)
	var langs []string

	for marker, lang := range languageMarkers {
		// Handle glob patterns (e.g., *.csproj)
		if marker[0] == '*' {
			matches, err := filepath.Glob(filepath.Join(root, marker))
			if err == nil && len(matches) > 0 {
				if !seen[lang] {
					seen[lang] = true
					langs = append(langs, lang)
				}
			}
			continue
		}

		// Check for exact file match
		path := filepath.Join(root, marker)
		if _, err := os.Stat(path); err == nil {
			if !seen[lang] {
				seen[lang] = true
				langs = append(langs, lang)
			}
		}
	}

	return langs
}
