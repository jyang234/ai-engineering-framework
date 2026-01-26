//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/aef/edi/internal/testutil"
)

// getEdiBinary returns the path to the edi binary.
// It looks for the binary in common locations. The binary should be built
// before running integration tests (make test-integration handles this).
func getEdiBinary(t *testing.T) string {
	t.Helper()

	// Get the current working directory to construct absolute paths
	cwd, _ := os.Getwd()

	// Look for binary relative to test file location
	// Tests are in internal/integration/, binary is in bin/
	binPaths := []string{
		filepath.Join(cwd, "..", "..", "bin", "edi"),
		filepath.Join(cwd, "bin", "edi"),
		// Also check PATH
		"edi",
	}

	for _, binPath := range binPaths {
		absPath, _ := filepath.Abs(binPath)
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

func TestEDIInitProject(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Change to project directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(env.ProjectDir)

	// Remove .edi that was created by SetupTestEnv to test init fresh
	os.RemoveAll(env.ProjectEDI)

	ediBinary := getEdiBinary(t)

	// Test: Run edi init
	t.Run("InitCreatesStructure", func(t *testing.T) {
		cmd := exec.Command(ediBinary, "init")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("edi init failed: %v\nOutput: %s", err, output)
		}

		// Verify output message
		if !strings.Contains(string(output), "Initialized EDI in current project") {
			t.Errorf("Expected success message, got: %s", output)
		}

		// Verify directory structure
		expectedDirs := []string{
			".edi",
			".edi/history",
			".edi/tasks",
			".edi/recall",
		}

		for _, dir := range expectedDirs {
			path := filepath.Join(env.ProjectDir, dir)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected directory %s to exist", dir)
			}
		}

		// Verify config.yaml created
		configPath := filepath.Join(env.ProjectDir, ".edi", "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Expected config.yaml to exist")
		}

		// Verify profile.md created
		profilePath := filepath.Join(env.ProjectDir, ".edi", "profile.md")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			t.Error("Expected profile.md to exist")
		}

		// Verify profile.md has template content
		content, _ := os.ReadFile(profilePath)
		if !strings.Contains(string(content), "# Project Profile") {
			t.Error("Expected profile.md to contain template header")
		}
	})

	// Test: edi init fails if already initialized
	t.Run("InitFailsIfExists", func(t *testing.T) {
		cmd := exec.Command(ediBinary, "init")
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Error("Expected edi init to fail when .edi already exists")
		}

		if !strings.Contains(string(output), "already exists") {
			t.Errorf("Expected 'already exists' message, got: %s", output)
		}
	})

	// Test: edi init --force overwrites
	t.Run("InitForceOverwrites", func(t *testing.T) {
		// Modify profile to detect overwrite
		profilePath := filepath.Join(env.ProjectDir, ".edi", "profile.md")
		os.WriteFile(profilePath, []byte("# Custom Profile"), 0644)

		cmd := exec.Command(ediBinary, "init", "--force")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("edi init --force failed: %v\nOutput: %s", err, output)
		}

		// Verify profile was reset
		content, _ := os.ReadFile(profilePath)
		if !strings.Contains(string(content), "# Project Profile") {
			t.Error("Expected profile.md to be reset with --force")
		}
	})
}

func TestEDIInitGlobal(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	// Remove global .edi that was created by SetupTestEnv
	os.RemoveAll(env.GlobalDir)

	ediBinary := getEdiBinary(t)

	// Test: Run edi init --global
	t.Run("InitGlobalCreatesStructure", func(t *testing.T) {
		cmd := exec.Command(ediBinary, "init", "--global")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("edi init --global failed: %v\nOutput: %s", err, output)
		}

		// Verify output message
		if !strings.Contains(string(output), "Initialized global EDI at ~/.edi/") {
			t.Errorf("Expected success message, got: %s", output)
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
			path := filepath.Join(env.Home, dir)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected directory %s to exist", dir)
			}
		}

		// Verify agents were installed
		agentsDir := filepath.Join(env.Home, ".edi", "agents")
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			t.Fatalf("Failed to read agents directory: %v", err)
		}

		if len(entries) < 1 {
			t.Error("Expected at least one agent to be installed")
		}

		// Check for known agents
		foundCoder := false
		for _, entry := range entries {
			if entry.Name() == "coder.md" {
				foundCoder = true
				break
			}
		}
		if !foundCoder {
			t.Error("Expected coder.md agent to be installed")
		}

		// Verify config.yaml created
		configPath := filepath.Join(env.Home, ".edi", "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Expected config.yaml to exist")
		}
	})

	// Test: Subagents installed to Claude directory
	t.Run("SubagentsInstalledToClaude", func(t *testing.T) {
		claudeAgentsDir := filepath.Join(env.Home, ".claude", "agents")
		if _, err := os.Stat(claudeAgentsDir); os.IsNotExist(err) {
			t.Error("Expected Claude agents directory to exist")
		}

		// Verify at least some subagents were installed
		entries, err := os.ReadDir(claudeAgentsDir)
		if err != nil {
			t.Fatalf("Failed to read Claude agents directory: %v", err)
		}

		if len(entries) < 1 {
			t.Error("Expected at least one subagent to be installed")
		}
	})

	// Test: edi-core skill installed to Claude
	t.Run("EdiCoreSkillInstalled", func(t *testing.T) {
		skillPath := filepath.Join(env.Home, ".claude", "skills", "edi-core", "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			t.Error("Expected edi-core skill to be installed")
		}
	})
}

func TestRecallServerStartup(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	dbPath := filepath.Join(env.ProjectEDI, "recall", "test.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	ediBinary := getEdiBinary(t)

	// Test: Start recall-server and verify it responds
	t.Run("ServerStartsAndResponds", func(t *testing.T) {
		client, err := testutil.NewMCPTestClient(
			ediBinary,
			"recall-server",
			"--project-db", dbPath,
			"--session-id", "startup-test-001",
		)
		if err != nil {
			t.Fatalf("Failed to create MCP client: %v", err)
		}
		defer client.Close()

		// Send initialize
		result, err := client.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if result.ServerInfo.Name != "recall" {
			t.Errorf("Expected server name 'recall', got '%s'", result.ServerInfo.Name)
		}
	})

	// Test: Server returns all 5 tools
	t.Run("ServerReturnsAllTools", func(t *testing.T) {
		client, err := testutil.NewMCPTestClient(
			ediBinary,
			"recall-server",
			"--project-db", dbPath,
			"--session-id", "startup-test-002",
		)
		if err != nil {
			t.Fatalf("Failed to create MCP client: %v", err)
		}
		defer client.Close()

		if _, err := client.Initialize(); err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		tools, err := client.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 5 {
			t.Errorf("Expected 5 tools, got %d", len(tools))
		}

		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name] = true
		}

		expectedTools := []string{
			"recall_search",
			"recall_get",
			"recall_add",
			"recall_feedback",
			"flight_recorder_log",
		}

		for _, name := range expectedTools {
			if !toolNames[name] {
				t.Errorf("Expected tool '%s' not found", name)
			}
		}
	})

	// Test: Server shuts down gracefully on stdin close
	t.Run("ServerGracefulShutdown", func(t *testing.T) {
		client, err := testutil.NewMCPTestClient(
			ediBinary,
			"recall-server",
			"--project-db", dbPath,
			"--session-id", "startup-test-003",
		)
		if err != nil {
			t.Fatalf("Failed to create MCP client: %v", err)
		}

		if _, err := client.Initialize(); err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Close should complete without error
		errCh := make(chan error, 1)
		go func() {
			errCh <- client.Close()
		}()

		select {
		case err := <-errCh:
			if err != nil {
				// Exit error is expected when stdin closes
				t.Logf("Server exit (expected): %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Server did not shut down within timeout")
		}
	})
}

func TestRecallServerSessionIDRequired(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	dbPath := filepath.Join(env.ProjectEDI, "recall", "test.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	ediBinary := getEdiBinary(t)

	// Test: Server fails without session-id
	t.Run("FailsWithoutSessionID", func(t *testing.T) {
		cmd := exec.Command(ediBinary, "recall-server", "--project-db", dbPath)
		output, err := cmd.CombinedOutput()

		if err == nil {
			t.Error("Expected recall-server to fail without --session-id")
		}

		if !strings.Contains(string(output), "session-id") {
			t.Errorf("Expected error message about session-id, got: %s", output)
		}
	})
}
