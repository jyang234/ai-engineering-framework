package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProject(t *testing.T) {
	// Cannot use t.Parallel() - modifies working directory
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Save and restore working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Run init
	if err := initProject(false); err != nil {
		t.Fatalf("initProject failed: %v", err)
	}

	// Verify directory structure
	expectedDirs := []string{
		".edi",
		".edi/history",
		".edi/tasks",
		".edi/recall",
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist", dir)
		}
	}

	// Verify config.yaml was created
	configPath := filepath.Join(tmpDir, ".edi", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected .edi/config.yaml to exist")
	}

	// Verify profile.md was created
	profilePath := filepath.Join(tmpDir, ".edi", "profile.md")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Expected .edi/profile.md to exist")
	}

	// Verify profile has expected content
	profileContent, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("Failed to read profile.md: %v", err)
	}
	if len(profileContent) == 0 {
		t.Error("Expected profile.md to have content")
	}
}

func TestInitProjectAlreadyExists(t *testing.T) {
	// Cannot use t.Parallel() - modifies working directory
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Save and restore working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create .edi directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".edi"), 0755); err != nil {
		t.Fatalf("Failed to create .edi: %v", err)
	}

	// Should fail without --force
	if err := initProject(false); err == nil {
		t.Error("Expected initProject to fail when .edi exists")
	}

	// Should succeed with --force
	if err := initProject(true); err != nil {
		t.Errorf("Expected initProject with force to succeed: %v", err)
	}
}

func TestInitGlobal(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Run global init
	if err := initGlobal(false); err != nil {
		t.Fatalf("initGlobal failed: %v", err)
	}

	// Verify directory structure
	expectedDirs := []string{
		".edi",
		".edi/agents",
		".edi/commands",
		".edi/skills",
		".edi/recall",
		".edi/cache",
		".edi/logs",
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpHome, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist", dir)
		}
	}

	// Verify config.yaml was created
	configPath := filepath.Join(tmpHome, ".edi", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected ~/.edi/config.yaml to exist")
	}

	// Verify agents were installed
	agentsDir := filepath.Join(tmpHome, ".edi", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		t.Fatalf("Failed to read agents directory: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Expected agents to be installed")
	}

	// Verify commands were installed
	commandsDir := filepath.Join(tmpHome, ".edi", "commands")
	entries, err = os.ReadDir(commandsDir)
	if err != nil {
		t.Fatalf("Failed to read commands directory: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Expected commands to be installed")
	}

	// Verify edi-core skill was installed to Claude's skills directory
	skillPath := filepath.Join(tmpHome, ".claude", "skills", "edi-core", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("Expected ~/.claude/skills/edi-core/SKILL.md to exist")
	}

	// Verify subagents were installed to Claude's agents directory
	claudeAgentsDir := filepath.Join(tmpHome, ".claude", "agents")
	entries, err = os.ReadDir(claudeAgentsDir)
	if err != nil {
		t.Fatalf("Failed to read Claude agents directory: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Expected subagents to be installed to ~/.claude/agents/")
	}
}

func TestInitGlobalAlreadyExists(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create ~/.edi directory
	if err := os.MkdirAll(filepath.Join(tmpHome, ".edi"), 0755); err != nil {
		t.Fatalf("Failed to create .edi: %v", err)
	}

	// Should fail without --force
	if err := initGlobal(false); err == nil {
		t.Error("Expected initGlobal to fail when ~/.edi exists")
	}

	// Should succeed with --force
	if err := initGlobal(true); err != nil {
		t.Errorf("Expected initGlobal with force to succeed: %v", err)
	}
}

func TestExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Test existing directory
	if !exists(tmpDir) {
		t.Error("Expected exists to return true for existing directory")
	}

	// Test existing file
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !exists(filePath) {
		t.Error("Expected exists to return true for existing file")
	}

	// Test non-existent path
	if exists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("Expected exists to return false for non-existent path")
	}
}

func TestWriteProfileTemplate(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	profilePath := filepath.Join(tmpDir, "profile.md")
	if err := writeProfileTemplate(profilePath); err != nil {
		t.Fatalf("writeProfileTemplate failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Fatal("Expected profile.md to exist")
	}

	// Verify content
	content, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("Failed to read profile.md: %v", err)
	}

	// Check for expected sections
	expectedSections := []string{
		"# Project Profile",
		"## Overview",
		"## Architecture",
		"## Tech Stack",
		"## Conventions",
		"## Key Decisions",
		"## Getting Started",
	}

	contentStr := string(content)
	for _, section := range expectedSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("Expected profile template to contain %q", section)
		}
	}
}
