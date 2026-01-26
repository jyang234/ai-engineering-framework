package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/aef/edi/internal/briefing"
	"github.com/anthropics/aef/edi/internal/config"
	"github.com/anthropics/aef/edi/internal/launch"
)

func TestBuildLaunchContext(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create minimal global structure
	globalDir := filepath.Join(tmpHome, ".edi")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a basic config
	cfg := &config.Config{
		Agent: "coder",
		Recall: config.RecallConfig{
			Enabled: true,
		},
	}

	// Create a basic briefing
	brief := &briefing.Briefing{
		ProjectContext: "# Test Project\n\nA test project.",
		HasProfile:     true,
	}

	// Build context
	contextPath, err := launch.BuildContext(cfg, "test-session-123", brief, "test-project")
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	// Verify context file was created
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		t.Fatal("Expected context file to be created")
	}

	// Read and verify content
	content, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("Failed to read context file: %v", err)
	}

	contentStr := string(content)

	// Verify EDI identity is present
	if !strings.Contains(contentStr, "EDI - Enhanced Development Intelligence") {
		t.Error("Expected context to contain EDI identity")
	}

	// Verify session ID is present
	if !strings.Contains(contentStr, "test-session-123") {
		t.Error("Expected context to contain session ID")
	}

	// Verify agent mode is present
	if !strings.Contains(contentStr, "Current Mode: coder") {
		t.Error("Expected context to contain agent mode")
	}

	// Verify RECALL instructions are present (since enabled)
	if !strings.Contains(contentStr, "RECALL Knowledge Base") {
		t.Error("Expected context to contain RECALL instructions when enabled")
	}

	// Verify slash commands are present
	if !strings.Contains(contentStr, "EDI Slash Commands") {
		t.Error("Expected context to contain slash command instructions")
	}

	// Verify briefing is included
	if !strings.Contains(contentStr, "Test Project") {
		t.Error("Expected context to include briefing content")
	}
}

func TestBuildLaunchContextWithoutRECALL(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create minimal global structure
	globalDir := filepath.Join(tmpHome, ".edi")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config with RECALL disabled
	cfg := &config.Config{
		Agent: "coder",
		Recall: config.RecallConfig{
			Enabled: false,
		},
	}

	// Build context without briefing
	contextPath, err := launch.BuildContext(cfg, "test-session-456", nil, "test-project")
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("Failed to read context file: %v", err)
	}

	contentStr := string(content)

	// RECALL instructions should NOT be present
	if strings.Contains(contentStr, "RECALL Knowledge Base") {
		t.Error("Expected context to NOT contain RECALL instructions when disabled")
	}
}

func TestBuildLaunchContextCreatesCache(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Don't create .edi directory - BuildContext should create cache

	cfg := &config.Config{
		Agent: "coder",
	}

	_, err := launch.BuildContext(cfg, "test-session", nil, "test-project")
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	// Verify cache directory was created
	cacheDir := filepath.Join(tmpHome, ".edi", "cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Expected cache directory to be created")
	}
}

func TestSlashCommandInstructions(t *testing.T) {
	// Cannot use t.Parallel() - modifies HOME env var
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create minimal structure
	if err := os.MkdirAll(filepath.Join(tmpHome, ".edi"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Agent: "coder",
	}

	contextPath, err := launch.BuildContext(cfg, "test", nil, "test")
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	content, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("Failed to read context: %v", err)
	}

	contentStr := string(content)

	// Verify all slash commands are documented
	expectedCommands := []string{
		"/plan",
		"/build",
		"/review",
		"/incident",
		"/task",
		"/end",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(contentStr, cmd) {
			t.Errorf("Expected slash command %s to be documented", cmd)
		}
	}

	// Verify aliases are documented
	expectedAliases := []string{
		"/architect",
		"/design",
		"/code",
		"/implement",
		"/check",
		"/debug",
		"/fix",
	}

	for _, alias := range expectedAliases {
		if !strings.Contains(contentStr, alias) {
			t.Errorf("Expected alias %s to be documented", alias)
		}
	}
}
