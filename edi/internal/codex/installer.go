package codex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckBinaryExists checks if the recall-mcp binary exists at ~/.edi/bin/recall-mcp
func CheckBinaryExists() (bool, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, ""
	}
	path := filepath.Join(home, ".edi", "bin", "recall-mcp")
	if _, err := os.Stat(path); err == nil {
		return true, path
	}
	return false, path
}

// DetectSource finds the codex/ directory relative to the EDI binary or CWD.
// Returns the absolute path to the codex directory, or empty string if not found.
func DetectSource() string {
	// Try relative to CWD (dev workflow: running from repo root)
	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "codex")
		if isCodexDir(candidate) {
			return candidate
		}
	}

	// Try relative to the EDI binary location
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		// Binary might be in edi/bin/ or ~/.edi/bin/, so check a few levels up
		for _, rel := range []string{"../codex", "../../codex", "../../../codex"} {
			candidate := filepath.Join(exeDir, rel)
			if abs, err := filepath.Abs(candidate); err == nil && isCodexDir(abs) {
				return abs
			}
		}
	}

	return ""
}

// isCodexDir checks if a directory looks like the codex source directory
func isCodexDir(path string) bool {
	// Must have a Makefile and cmd/recall-mcp/
	makefile := filepath.Join(path, "Makefile")
	cmdDir := filepath.Join(path, "cmd", "recall-mcp")
	_, errMake := os.Stat(makefile)
	_, errCmd := os.Stat(cmdDir)
	return errMake == nil && errCmd == nil
}

// Build runs `make build` in the codex directory.
func Build(codexPath string) error {
	cmd := exec.Command("make", "build")
	cmd.Dir = codexPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex build failed: %w", err)
	}
	return nil
}

// InstallBinary copies the built recall-mcp binary from codex/bin/ to ~/.edi/bin/
func InstallBinary(codexPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	src := filepath.Join(codexPath, "bin", "recall-mcp")
	dst := filepath.Join(home, ".edi", "bin", "recall-mcp")

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("built binary not found at %s", src)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Copy file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	return nil
}

// CheckOllama checks if Ollama is available and nomic-embed-text is pulled.
// Returns (ollamaAvailable, modelAvailable, error).
func CheckOllama() (bool, bool) {
	// Check if ollama binary exists
	if _, err := exec.LookPath("ollama"); err != nil {
		return false, false
	}

	// Check if nomic-embed-text model is available
	cmd := exec.Command("ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return true, false
	}

	// Simple substring check for the model name
	return true, contains(string(output), "nomic-embed-text")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
