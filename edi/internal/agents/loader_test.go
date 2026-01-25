package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentFile(t *testing.T) {
	content := []byte(`---
name: test-agent
description: A test agent
tools:
  - Read
  - Write
skills:
  - edi-core
---

# Test Agent

This is the system prompt content.

## Behaviors

- Be helpful
- Be accurate
`)

	agent, body, err := parseAgentFile(content)
	if err != nil {
		t.Fatalf("parseAgentFile failed: %v", err)
	}

	if agent.Name != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.Name)
	}

	if agent.Description != "A test agent" {
		t.Errorf("Expected description 'A test agent', got '%s'", agent.Description)
	}

	if len(agent.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(agent.Tools))
	}

	if len(agent.Skills) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(agent.Skills))
	}

	if body == "" {
		t.Error("Expected non-empty body")
	}
}

func TestParseAgentFileNoFrontmatter(t *testing.T) {
	content := []byte(`# Test Agent

This is just a system prompt without frontmatter.
`)

	agent, body, err := parseAgentFile(content)
	if err != nil {
		t.Fatalf("parseAgentFile failed: %v", err)
	}

	// Agent should have empty fields
	if agent.Name != "" {
		t.Errorf("Expected empty name, got '%s'", agent.Name)
	}

	// Body should be the full content
	if body != string(content) {
		t.Errorf("Expected body to be full content")
	}
}

func TestLoadAgentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	agentContent := `---
name: coder
description: Implementation mode
---

# Coder Agent

Write code.
`

	path := filepath.Join(tmpDir, "coder.md")
	if err := os.WriteFile(path, []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}

	agent, err := loadAgentFile(path)
	if err != nil {
		t.Fatalf("loadAgentFile failed: %v", err)
	}

	if agent.Name != "coder" {
		t.Errorf("Expected name 'coder', got '%s'", agent.Name)
	}

	if agent.SystemPrompt == "" {
		t.Error("Expected non-empty system prompt")
	}
}
