package launch

import (
	"os"
	"path/filepath"
	"testing"
)

// withWorkingDir sets up HOME and working directory for tests that need them.
// It saves and restores both after the test function completes.
func withWorkingDir(t *testing.T, home, project string, fn func()) {
	t.Helper()

	t.Setenv("HOME", home)

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(project); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	fn()
}

func TestInstallCommands(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME and working directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	withWorkingDir(t, tmpHome, tmpProject, func() {
		// Create source commands directory
		srcDir := filepath.Join(tmpHome, ".edi", "commands")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create test command files
		testCommands := map[string]string{
			"plan.md":   "# Plan Command\n\nSwitch to planning mode.",
			"build.md":  "# Build Command\n\nSwitch to build mode.",
			"review.md": "# Review Command\n\nSwitch to review mode.",
		}

		for name, content := range testCommands {
			path := filepath.Join(srcDir, name)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create command %s: %v", name, err)
			}
		}

		// Run InstallCommands
		if err := InstallCommands(); err != nil {
			t.Fatalf("InstallCommands failed: %v", err)
		}

		// Verify commands were copied to .claude/commands/
		dstDir := filepath.Join(tmpProject, ".claude", "commands")
		entries, err := os.ReadDir(dstDir)
		if err != nil {
			t.Fatalf("Failed to read destination directory: %v", err)
		}

		if len(entries) != 3 {
			t.Errorf("Expected 3 commands, got %d", len(entries))
		}

		// Verify content matches
		for name, expectedContent := range testCommands {
			dstPath := filepath.Join(dstDir, name)
			actualContent, err := os.ReadFile(dstPath)
			if err != nil {
				t.Errorf("Failed to read copied command %s: %v", name, err)
				continue
			}
			if string(actualContent) != expectedContent {
				t.Errorf("Content mismatch for %s", name)
			}
		}
	})
}

func TestInstallCommandsSkipsUnchanged(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME and working directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	withWorkingDir(t, tmpHome, tmpProject, func() {
		// Create source command
		srcDir := filepath.Join(tmpHome, ".edi", "commands")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := "# Test Command\n\nSome content."
		if err := os.WriteFile(filepath.Join(srcDir, "test.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// First install
		if err := InstallCommands(); err != nil {
			t.Fatalf("First InstallCommands failed: %v", err)
		}

		// Get modification time of destination
		dstPath := filepath.Join(tmpProject, ".claude", "commands", "test.md")
		info1, err := os.Stat(dstPath)
		if err != nil {
			t.Fatal(err)
		}
		modTime1 := info1.ModTime()

		// Second install (should skip since unchanged)
		if err := InstallCommands(); err != nil {
			t.Fatalf("Second InstallCommands failed: %v", err)
		}

		// Mod time should be unchanged (file was not rewritten)
		info2, err := os.Stat(dstPath)
		if err != nil {
			t.Fatal(err)
		}
		modTime2 := info2.ModTime()

		if !modTime1.Equal(modTime2) {
			t.Error("Expected file to not be rewritten when unchanged")
		}
	})
}

func TestInstallCommandsUpdatesChanged(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME and working directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	withWorkingDir(t, tmpHome, tmpProject, func() {
		// Create source command
		srcDir := filepath.Join(tmpHome, ".edi", "commands")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		srcPath := filepath.Join(srcDir, "test.md")
		if err := os.WriteFile(srcPath, []byte("Version 1"), 0644); err != nil {
			t.Fatal(err)
		}

		// First install
		if err := InstallCommands(); err != nil {
			t.Fatal(err)
		}

		// Update source
		if err := os.WriteFile(srcPath, []byte("Version 2"), 0644); err != nil {
			t.Fatal(err)
		}

		// Second install
		if err := InstallCommands(); err != nil {
			t.Fatal(err)
		}

		// Verify destination was updated
		dstPath := filepath.Join(tmpProject, ".claude", "commands", "test.md")
		content, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatal(err)
		}

		if string(content) != "Version 2" {
			t.Errorf("Expected 'Version 2', got %q", string(content))
		}
	})
}

func TestInstallCommandsNoSourceDir(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME and working directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	withWorkingDir(t, tmpHome, tmpProject, func() {
		// Don't create source directory - should not error
		if err := InstallCommands(); err != nil {
			t.Errorf("InstallCommands should not error when source dir missing: %v", err)
		}
	})
}

func TestInstallCommandsSkipsNonMarkdown(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME and working directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	withWorkingDir(t, tmpHome, tmpProject, func() {
		// Create source with mixed files
		srcDir := filepath.Join(tmpHome, ".edi", "commands")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Markdown file
		if err := os.WriteFile(filepath.Join(srcDir, "command.md"), []byte("# Command"), 0644); err != nil {
			t.Fatal(err)
		}

		// Non-markdown files (should be skipped)
		if err := os.WriteFile(filepath.Join(srcDir, "readme.txt"), []byte("readme"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "config.yaml"), []byte("config"), 0644); err != nil {
			t.Fatal(err)
		}

		// Subdirectory (should be skipped)
		if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		if err := InstallCommands(); err != nil {
			t.Fatal(err)
		}

		// Only markdown should be copied
		dstDir := filepath.Join(tmpProject, ".claude", "commands")
		entries, err := os.ReadDir(dstDir)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 1 {
			t.Errorf("Expected 1 file (only .md), got %d", len(entries))
		}

		if entries[0].Name() != "command.md" {
			t.Errorf("Expected command.md, got %s", entries[0].Name())
		}
	})
}

func TestNeedsCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		srcData string
		dstData *string // nil means destination doesn't exist
		want    bool
	}{
		{
			name:    "destination doesn't exist",
			srcData: "content",
			dstData: nil,
			want:    true,
		},
		{
			name:    "identical content",
			srcData: "content",
			dstData: ptr("content"),
			want:    false,
		},
		{
			name:    "different content",
			srcData: "content",
			dstData: ptr("different"),
			want:    true,
		},
		{
			name:    "empty destination",
			srcData: "content",
			dstData: ptr(""),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			srcPath := filepath.Join(tmpDir, "src.txt")
			dstPath := filepath.Join(tmpDir, "dst.txt")

			if err := os.WriteFile(srcPath, []byte(tt.srcData), 0644); err != nil {
				t.Fatal(err)
			}

			if tt.dstData != nil {
				if err := os.WriteFile(dstPath, []byte(*tt.dstData), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := needsCopy(srcPath, dstPath)
			if got != tt.want {
				t.Errorf("needsCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ptr returns a pointer to the given string (helper for table-driven tests)
func ptr(s string) *string {
	return &s
}

func TestFileHash(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Test: same content produces same hash
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(path1, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	hash1, err := fileHash(path1)
	if err != nil {
		t.Fatalf("fileHash failed: %v", err)
	}

	hash2, err := fileHash(path2)
	if err != nil {
		t.Fatalf("fileHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Expected same content to produce same hash")
	}

	// Test: different content produces different hash
	path3 := filepath.Join(tmpDir, "file3.txt")
	if err := os.WriteFile(path3, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	hash3, err := fileHash(path3)
	if err != nil {
		t.Fatalf("fileHash failed: %v", err)
	}

	if hash1 == hash3 {
		t.Error("Expected different content to produce different hash")
	}

	// Test: non-existent file returns error
	_, err = fileHash(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")
	content := "Hello, World!\nLine 2\nLine 3"

	// Create source file
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	actual, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(actual) != content {
		t.Errorf("Content mismatch: expected %q, got %q", content, string(actual))
	}
}

func TestCopyFileSourceNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	err := copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("Expected error when source doesn't exist")
	}
}
