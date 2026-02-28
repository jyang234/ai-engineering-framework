package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// GetEdiBinary returns the path to the edi binary.
// It looks for the binary in common locations. The binary should be built
// before running integration tests (make test-integration handles this).
func GetEdiBinary(t *testing.T) string {
	t.Helper()

	// Get the current working directory to construct absolute paths
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Look for binary relative to test file location
	// Tests may be in internal/<pkg>/, binary is in bin/
	binPaths := []string{
		filepath.Join(cwd, "..", "..", "bin", "edi"),
		filepath.Join(cwd, "bin", "edi"),
	}

	for _, binPath := range binPaths {
		absPath, err := filepath.Abs(binPath)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	// Try to find via PATH
	if path, err := exec.LookPath("edi"); err == nil {
		return path
	}

	t.Fatal("edi binary not found. Run 'make build' first or ensure edi is in PATH")
	return ""
}
