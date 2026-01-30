package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aef/edi/internal/config"
)

// MCPServerConfig represents a single MCP server configuration
type MCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPConfig represents the .mcp.json file structure
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// GetRecallMCPConfig returns the MCP server configuration for RECALL based on the backend setting
func GetRecallMCPConfig(cfg *config.Config, sessionID string) MCPServerConfig {
	if cfg.Recall.Backend == "codex" {
		return getCodexMCPConfig(cfg, sessionID)
	}
	return getV0MCPConfig(cfg, sessionID)
}

// getV0MCPConfig returns the configuration for RECALL v0 (SQLite FTS)
func getV0MCPConfig(cfg *config.Config, sessionID string) MCPServerConfig {
	home, _ := os.UserHomeDir()
	ediBinary := findEdiBinary()

	return MCPServerConfig{
		Type:    "stdio",
		Command: ediBinary,
		Args: []string{
			"recall-server",
			"--session-id", sessionID,
			"--global-db", filepath.Join(home, ".edi", "recall", "global.db"),
		},
	}
}

// getCodexMCPConfig returns the configuration for Codex v1 (hybrid vector search)
func getCodexMCPConfig(cfg *config.Config, sessionID string) MCPServerConfig {
	home, _ := os.UserHomeDir()

	// Determine binary path
	binaryPath := cfg.Codex.BinaryPath
	if binaryPath == "" {
		binaryPath = filepath.Join(home, ".edi", "bin", "recall-mcp")
	}

	// Build environment variables
	env := map[string]string{
		"EDI_SESSION_ID": sessionID,
	}

	// Add Codex configuration
	if cfg.Codex.ModelsPath != "" {
		env["CODEX_MODELS_PATH"] = expandPath(cfg.Codex.ModelsPath)
	}
	if cfg.Codex.MetadataDB != "" {
		env["CODEX_METADATA_DB"] = expandPath(cfg.Codex.MetadataDB)
	}

	// Pass through API keys and embedding config from environment
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		env["ANTHROPIC_API_KEY"] = "${ANTHROPIC_API_KEY}"
	}
	if url := os.Getenv("LOCAL_EMBEDDING_URL"); url != "" {
		env["LOCAL_EMBEDDING_URL"] = "${LOCAL_EMBEDDING_URL}"
	}
	if model := os.Getenv("LOCAL_EMBEDDING_MODEL"); model != "" {
		env["LOCAL_EMBEDDING_MODEL"] = "${LOCAL_EMBEDDING_MODEL}"
	}
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		env["CODEX_API_KEY"] = "${CODEX_API_KEY}"
	}

	return MCPServerConfig{
		Type:    "stdio",
		Command: binaryPath,
		Env:     env,
	}
}

// WriteMCPConfig writes the MCP configuration to .mcp.json in the project directory
func WriteMCPConfig(projectDir string, cfg *config.Config, sessionID string) error {
	if !cfg.Recall.Enabled {
		return nil // No MCP config needed if RECALL is disabled
	}

	mcpConfig := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"recall": GetRecallMCPConfig(cfg, sessionID),
		},
	}

	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	mcpPath := filepath.Join(projectDir, ".mcp.json")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	return nil
}

// ReadMCPConfig reads an existing .mcp.json file
func ReadMCPConfig(projectDir string) (*MCPConfig, error) {
	mcpPath := filepath.Join(projectDir, ".mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPConfig{MCPServers: make(map[string]MCPServerConfig)}, nil
		}
		return nil, fmt.Errorf("failed to read .mcp.json: %w", err)
	}

	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse .mcp.json: %w", err)
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServerConfig)
	}

	return &cfg, nil
}

// UpdateMCPConfig updates the RECALL server in an existing .mcp.json, preserving other servers
func UpdateMCPConfig(projectDir string, cfg *config.Config, sessionID string) error {
	if !cfg.Recall.Enabled {
		return nil
	}

	// Read existing config
	mcpCfg, err := ReadMCPConfig(projectDir)
	if err != nil {
		return err
	}

	// Update RECALL server configuration
	mcpCfg.MCPServers["recall"] = GetRecallMCPConfig(cfg, sessionID)

	// Write back
	data, err := json.MarshalIndent(mcpCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	mcpPath := filepath.Join(projectDir, ".mcp.json")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	return nil
}

// findEdiBinary returns the path to the edi binary
func findEdiBinary() string {
	// Check if running from installed location
	home, _ := os.UserHomeDir()
	installed := filepath.Join(home, ".edi", "bin", "edi")
	if _, err := os.Stat(installed); err == nil {
		return installed
	}

	// Fall back to PATH lookup
	return "edi"
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// ValidateCodexRequirements checks if Codex backend requirements are met
func ValidateCodexRequirements(cfg *config.Config) error {
	if cfg.Recall.Backend != "codex" {
		return nil
	}

	// Check for binary
	home, _ := os.UserHomeDir()
	binaryPath := cfg.Codex.BinaryPath
	if binaryPath == "" {
		binaryPath = filepath.Join(home, ".edi", "bin", "recall-mcp")
	}

	if _, err := os.Stat(expandPath(binaryPath)); os.IsNotExist(err) {
		return fmt.Errorf("codex binary not found at %s. Run 'make build' in codex/ directory and copy to ~/.edi/bin/", binaryPath)
	}

	return nil
}
