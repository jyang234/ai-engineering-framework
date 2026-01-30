package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

func TestGetRecallMCPConfig_V0(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: true,
			Backend: "v0",
		},
	}

	mcpCfg := GetRecallMCPConfig(cfg, "test-session-123")

	if mcpCfg.Type != "stdio" {
		t.Errorf("Expected type 'stdio', got '%s'", mcpCfg.Type)
	}

	// V0 should use edi binary with recall-server subcommand
	if !strings.Contains(mcpCfg.Command, "edi") {
		t.Errorf("Expected command to contain 'edi', got '%s'", mcpCfg.Command)
	}

	if len(mcpCfg.Args) < 2 {
		t.Fatalf("Expected at least 2 args, got %d", len(mcpCfg.Args))
	}

	if mcpCfg.Args[0] != "recall-server" {
		t.Errorf("Expected first arg 'recall-server', got '%s'", mcpCfg.Args[0])
	}

	// Check session-id is passed
	hasSessionID := false
	for i, arg := range mcpCfg.Args {
		if arg == "--session-id" && i+1 < len(mcpCfg.Args) && mcpCfg.Args[i+1] == "test-session-123" {
			hasSessionID = true
			break
		}
	}
	if !hasSessionID {
		t.Error("Expected --session-id test-session-123 in args")
	}
}

func TestGetRecallMCPConfig_Codex(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: true,
			Backend: "codex",
		},
		Codex: config.CodexConfig{},
	}

	mcpCfg := GetRecallMCPConfig(cfg, "test-session-456")

	if mcpCfg.Type != "stdio" {
		t.Errorf("Expected type 'stdio', got '%s'", mcpCfg.Type)
	}

	// Codex should use recall-mcp binary
	if !strings.Contains(mcpCfg.Command, "recall-mcp") {
		t.Errorf("Expected command to contain 'recall-mcp', got '%s'", mcpCfg.Command)
	}

	// Codex uses env vars instead of args
	if mcpCfg.Env == nil {
		t.Fatal("Expected Env to be set for codex backend")
	}

	if mcpCfg.Env["EDI_SESSION_ID"] != "test-session-456" {
		t.Errorf("Expected EDI_SESSION_ID 'test-session-456', got '%s'", mcpCfg.Env["EDI_SESSION_ID"])
	}

	// Verify stale env vars are NOT present
	if _, ok := mcpCfg.Env["QDRANT_ADDR"]; ok {
		t.Error("QDRANT_ADDR should not be set (removed)")
	}

	if _, ok := mcpCfg.Env["CODEX_COLLECTION"]; ok {
		t.Error("CODEX_COLLECTION should not be set (removed)")
	}
}

func TestGetRecallMCPConfig_CodexWithCustomBinary(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: true,
			Backend: "codex",
		},
		Codex: config.CodexConfig{
			BinaryPath: "/custom/path/recall-mcp",
		},
	}

	mcpCfg := GetRecallMCPConfig(cfg, "test-session")

	if mcpCfg.Command != "/custom/path/recall-mcp" {
		t.Errorf("Expected custom binary path, got '%s'", mcpCfg.Command)
	}
}

func TestWriteMCPConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: true,
			Backend: "v0",
		},
	}

	err := WriteMCPConfig(tmpDir, cfg, "test-session")
	if err != nil {
		t.Fatalf("WriteMCPConfig failed: %v", err)
	}

	// Verify file was created
	mcpPath := filepath.Join(tmpDir, ".mcp.json")
	content, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("Failed to read .mcp.json: %v", err)
	}

	// Parse and verify structure
	var mcpCfg MCPConfig
	if err := json.Unmarshal(content, &mcpCfg); err != nil {
		t.Fatalf("Failed to parse .mcp.json: %v", err)
	}

	if _, exists := mcpCfg.MCPServers["recall"]; !exists {
		t.Error("Expected 'recall' server in MCP config")
	}
}

func TestWriteMCPConfig_DisabledRecall(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: false,
		},
	}

	err := WriteMCPConfig(tmpDir, cfg, "test-session")
	if err != nil {
		t.Fatalf("WriteMCPConfig failed: %v", err)
	}

	// File should not be created when RECALL is disabled
	mcpPath := filepath.Join(tmpDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Error("Expected .mcp.json to not be created when RECALL is disabled")
	}
}

func TestUpdateMCPConfig_PreservesOtherServers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create existing .mcp.json with another server
	existingConfig := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"other-server": {
				Type:    "stdio",
				Command: "/path/to/other",
			},
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	mcpPath := filepath.Join(tmpDir, ".mcp.json")
	os.WriteFile(mcpPath, data, 0644)

	// Update with RECALL config
	cfg := &config.Config{
		Recall: config.RecallConfig{
			Enabled: true,
			Backend: "v0",
		},
	}

	err := UpdateMCPConfig(tmpDir, cfg, "test-session")
	if err != nil {
		t.Fatalf("UpdateMCPConfig failed: %v", err)
	}

	// Read and verify both servers exist
	content, _ := os.ReadFile(mcpPath)
	var mcpCfg MCPConfig
	json.Unmarshal(content, &mcpCfg)

	if _, exists := mcpCfg.MCPServers["other-server"]; !exists {
		t.Error("Expected 'other-server' to be preserved")
	}

	if _, exists := mcpCfg.MCPServers["recall"]; !exists {
		t.Error("Expected 'recall' server to be added")
	}
}

func TestReadMCPConfig_NonExistent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	cfg, err := ReadMCPConfig(tmpDir)
	if err != nil {
		t.Fatalf("ReadMCPConfig failed for non-existent file: %v", err)
	}

	if cfg.MCPServers == nil {
		t.Error("Expected MCPServers to be initialized")
	}

	if len(cfg.MCPServers) != 0 {
		t.Error("Expected empty MCPServers for non-existent file")
	}
}

func TestExpandPath(t *testing.T) {
	t.Parallel()

	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/.edi/models", filepath.Join(home, ".edi/models")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tc := range tests {
		result := expandPath(tc.input)
		if result != tc.expected {
			t.Errorf("expandPath(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestValidateCodexRequirements_V0Backend(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Recall: config.RecallConfig{
			Backend: "v0",
		},
	}

	// Should pass without error for v0 backend
	err := ValidateCodexRequirements(cfg)
	if err != nil {
		t.Errorf("Expected no error for v0 backend, got: %v", err)
	}
}
