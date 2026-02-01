package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestDoctorOutputFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	output := captureOutput(func() {
		_ = runDoctor(nil, nil)
	})

	// Should contain section headers
	for _, section := range []string{"Global installation:", "RECALL backend:", "Claude Code:", "Project (current directory):"} {
		if !strings.Contains(output, section) {
			t.Errorf("expected output to contain section %q", section)
		}
	}

	// Should contain results summary
	if !strings.Contains(output, "Results:") {
		t.Error("expected output to contain Results summary")
	}

	// Should contain pass/fail markers
	if !strings.Contains(output, "✓") && !strings.Contains(output, "✗") {
		t.Error("expected output to contain ✓ or ✗ markers")
	}
}

func TestDoctorPassesWithSetup(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create global structure
	ediHome := filepath.Join(tmpHome, ".edi")
	for _, dir := range []string{"agents", "commands", "recall"} {
		os.MkdirAll(filepath.Join(ediHome, dir), 0755)
	}
	os.WriteFile(filepath.Join(ediHome, "config.yaml"), []byte("recall:\n  enabled: true\n"), 0644)

	output := captureOutput(func() {
		_ = runDoctor(nil, nil)
	})

	// Global checks should pass
	if !strings.Contains(output, "✓ ~/.edi/ directory") {
		t.Error("expected ~/.edi/ directory check to pass")
	}
	if !strings.Contains(output, "✓ ~/.edi/config.yaml") {
		t.Error("expected config.yaml check to pass")
	}
}

func TestDoctorFailsWithoutSetup(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	output := captureOutput(func() {
		_ = runDoctor(nil, nil)
	})

	// Should show failures for missing dirs
	if !strings.Contains(output, "✗ ~/.edi/ directory") {
		t.Error("expected ~/.edi/ directory check to fail")
	}
}

func TestExpandHomePath(t *testing.T) {
	t.Parallel()

	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		got := expandHomePath(tt.input)
		if got != tt.expected {
			t.Errorf("expandHomePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
