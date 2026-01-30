package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckBinaryExists_NotFound(t *testing.T) {
	exists, path := CheckBinaryExists()
	if path == "" {
		t.Fatal("expected non-empty path even when binary doesn't exist")
	}
	// We can't guarantee the binary exists or not in test env,
	// but we can verify the path is well-formed
	if filepath.Base(path) != "recall-mcp" {
		t.Errorf("expected path to end in recall-mcp, got %s", path)
	}
	_ = exists // value depends on test environment
}

func TestIsCodexDir(t *testing.T) {
	// Create a temp dir that looks like codex
	tmp := t.TempDir()
	if isCodexDir(tmp) {
		t.Error("empty dir should not be detected as codex dir")
	}

	// Create Makefile and cmd/recall-mcp/
	os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("build:"), 0644)
	os.MkdirAll(filepath.Join(tmp, "cmd", "recall-mcp"), 0755)

	if !isCodexDir(tmp) {
		t.Error("dir with Makefile and cmd/recall-mcp should be detected as codex dir")
	}
}

func TestDetectSource_FromCWD(t *testing.T) {
	// DetectSource looks at CWD; we can't easily control this in tests,
	// but we can verify it returns a string (possibly empty)
	result := DetectSource()
	if result != "" {
		if !isCodexDir(result) {
			t.Errorf("DetectSource returned %s but it's not a valid codex dir", result)
		}
	}
}

func TestInstallBinary_MissingSource(t *testing.T) {
	err := InstallBinary("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing source binary")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"nomic-embed-text:latest", "nomic-embed-text", true},
		{"some other model", "nomic-embed-text", false},
		{"", "nomic-embed-text", false},
		{"nomic-embed-text", "", true},
	}
	for _, tt := range tests {
		if got := contains(tt.s, tt.substr); got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}
