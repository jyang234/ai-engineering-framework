// Package testutil provides reusable test utilities for EDI integration tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnv provides access to isolated test directories
type TestEnv struct {
	Home       string // Mocked HOME directory
	ProjectDir string // Test project directory
	GlobalDir  string // ~/.edi equivalent
	ProjectEDI string // .edi in project
	t          *testing.T
}

// SetupTestEnv creates an isolated test environment with mocked HOME.
// Uses t.TempDir() for automatic cleanup and t.Setenv() for automatic env restoration.
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create temp directories using t.TempDir() for automatic cleanup
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	// Create directory structure
	globalDir := filepath.Join(tmpHome, ".edi")
	projectEDI := filepath.Join(tmpProject, ".edi")

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("Failed to create global .edi: %v", err)
	}

	if err := os.MkdirAll(projectEDI, 0755); err != nil {
		t.Fatalf("Failed to create project .edi: %v", err)
	}

	// Set HOME to temp directory (auto-restored after test)
	t.Setenv("HOME", tmpHome)

	return &TestEnv{
		Home:       tmpHome,
		ProjectDir: tmpProject,
		GlobalDir:  globalDir,
		ProjectEDI: projectEDI,
		t:          t,
	}
}

// CreateFile creates a file with the given content in the test environment.
func (e *TestEnv) CreateFile(path, content string) {
	e.t.Helper()

	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(e.ProjectDir, path)
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write file %s: %v", fullPath, err)
	}
}

// CreateProjectFile creates a file relative to the project directory.
func (e *TestEnv) CreateProjectFile(relPath, content string) {
	e.t.Helper()
	e.CreateFile(filepath.Join(e.ProjectDir, relPath), content)
}

// CreateGlobalFile creates a file relative to the global .edi directory.
func (e *TestEnv) CreateGlobalFile(relPath, content string) {
	e.t.Helper()
	e.CreateFile(filepath.Join(e.GlobalDir, relPath), content)
}

// ReadFile reads a file from the test environment.
func (e *TestEnv) ReadFile(path string) string {
	e.t.Helper()

	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(e.ProjectDir, path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		e.t.Fatalf("Failed to read file %s: %v", fullPath, err)
	}
	return string(data)
}

// FileExists checks if a file exists in the test environment.
func (e *TestEnv) FileExists(path string) bool {
	e.t.Helper()

	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(e.ProjectDir, path)
	}

	_, err := os.Stat(fullPath)
	return err == nil
}

// CreateClaudeTasksDir creates the .claude/tasks directory structure.
func (e *TestEnv) CreateClaudeTasksDir(sessionID string) string {
	e.t.Helper()

	dir := filepath.Join(e.Home, ".claude", "tasks", sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("Failed to create Claude tasks dir: %v", err)
	}
	return dir
}

// CreateEDITasksDir creates the .edi/tasks directory in the project.
func (e *TestEnv) CreateEDITasksDir() string {
	e.t.Helper()

	dir := filepath.Join(e.ProjectEDI, "tasks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.t.Fatalf("Failed to create EDI tasks dir: %v", err)
	}
	return dir
}

// SetupMinimalProject creates a minimal EDI project structure.
func (e *TestEnv) SetupMinimalProject() {
	e.t.Helper()

	// Create profile.md
	e.CreateProjectFile(".edi/profile.md", `# Test Project

## Overview
A test project for integration tests.

## Tech Stack
- Go
- SQLite
`)

	// Create config.yaml
	e.CreateProjectFile(".edi/config.yaml", `version: 1
`)
}

// SetupProjectWithTasks creates a project with task manifest.
func (e *TestEnv) SetupProjectWithTasks(manifestYAML string) {
	e.t.Helper()

	e.SetupMinimalProject()
	e.CreateEDITasksDir()
	e.CreateProjectFile(".edi/tasks/active.yaml", manifestYAML)
}
