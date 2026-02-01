package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	ensureGitignore(".ralph/")

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(data), ".ralph/") {
		t.Error("expected .gitignore to contain .ralph/")
	}
}

func TestEnsureGitignore_AppendsToExisting(t *testing.T) {
	tmpDir := t.TempDir()

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	ensureGitignore(".ralph/")

	data, _ := os.ReadFile(gitignorePath)
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("expected existing entries to be preserved")
	}
	if !strings.Contains(content, ".ralph/") {
		t.Error("expected .ralph/ to be appended")
	}
}

func TestEnsureGitignore_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(".ralph/\n"), 0644)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	ensureGitignore(".ralph/")

	data, _ := os.ReadFile(gitignorePath)
	if strings.Count(string(data), ".ralph/") != 1 {
		t.Error("expected .ralph/ to appear exactly once")
	}
}

func TestEnsureGitignore_NoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/"), 0644) // no trailing newline

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	ensureGitignore(".ralph/")

	data, _ := os.ReadFile(gitignorePath)
	lines := strings.Split(string(data), "\n")
	// Should have: "node_modules/", ".ralph/", ""
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == ".ralph/" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected .ralph/ on its own line, got: %q", string(data))
	}
}

func TestRunRalphInit_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	err := runRalphInit(nil, nil)
	if err != nil {
		t.Fatalf("runRalphInit failed: %v", err)
	}

	prdPath := filepath.Join(tmpDir, "PRD.json")
	if _, err := os.Stat(prdPath); os.IsNotExist(err) {
		t.Fatal("expected PRD.json to be created")
	}

	data, _ := os.ReadFile(prdPath)
	if !strings.Contains(string(data), "userStories") {
		t.Error("expected PRD.json to contain userStories")
	}
}

func TestRunRalphInit_ExistingFileNoForce(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "PRD.json"), []byte("{}"), 0644)

	ralphInitForce = false
	err := runRalphInit(nil, nil)
	if err == nil {
		t.Fatal("expected error when PRD.json exists without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected error to mention --force, got: %v", err)
	}
}

func TestRunRalphInit_ExistingFileWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "PRD.json"), []byte("{}"), 0644)

	ralphInitForce = true
	defer func() { ralphInitForce = false }()

	err := runRalphInit(nil, nil)
	if err != nil {
		t.Fatalf("expected force init to succeed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "PRD.json"))
	if !strings.Contains(string(data), "userStories") {
		t.Error("expected PRD.json to be overwritten with template")
	}
}

func TestRunRalph_NoPRD(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	ralphPRDPath = "PRD.json"
	err := runRalph(nil, nil)
	if err == nil {
		t.Fatal("expected error when PRD.json is missing")
	}
	if !strings.Contains(err.Error(), "PRD not found") {
		t.Errorf("expected 'PRD not found' error, got: %v", err)
	}
}

func TestRunRalph_PromptProvisioning(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	// Create a PRD so we get past the first check
	os.WriteFile("PRD.json", []byte(`{"userStories":[]}`), 0644)

	// Create a fake ralph.sh that exits immediately so we don't actually run the loop
	os.MkdirAll(".ralph", 0755)
	os.WriteFile(filepath.Join(".ralph", "ralph.sh"), []byte("#!/bin/bash\nexit 0\n"), 0755)

	ralphPRDPath = "PRD.json"
	ralphPromptPath = ""

	// No CWD PROMPT.md → should write embedded default to .ralph/PROMPT.md
	_ = runRalph(nil, nil)

	promptPath := filepath.Join(".ralph", "PROMPT.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		t.Error("expected .ralph/PROMPT.md to be provisioned")
	}
}

func TestRunRalph_CustomPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)

	// Create PRD and custom prompt
	os.WriteFile("PRD.json", []byte(`{"userStories":[]}`), 0644)
	os.WriteFile("custom-prompt.md", []byte("# Custom Prompt\nDo things differently."), 0644)

	ralphPRDPath = "PRD.json"
	ralphPromptPath = "custom-prompt.md"
	defer func() { ralphPromptPath = "" }()

	_ = runRalph(nil, nil)

	data, err := os.ReadFile(filepath.Join(".ralph", "PROMPT.md"))
	if err != nil {
		t.Fatalf("expected .ralph/PROMPT.md to exist: %v", err)
	}
	if !strings.Contains(string(data), "Custom Prompt") {
		t.Error("expected custom prompt content to be copied")
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := "hello world"
	os.WriteFile(src, []byte(content), 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}
