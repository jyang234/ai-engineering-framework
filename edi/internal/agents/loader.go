package agents

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Agent represents an EDI agent configuration
type Agent struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Tools        []string `yaml:"tools"`
	Skills       []string `yaml:"skills"`
	SystemPrompt string   `yaml:"-"` // Populated from markdown body
}

// Load loads an agent by name, checking project then global locations
func Load(name string) (*Agent, error) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	// Check project override first
	projectPath := filepath.Join(cwd, ".edi", "agents", name+".md")
	if agent, err := loadAgentFile(projectPath); err == nil {
		return agent, nil
	}

	// Check global
	globalPath := filepath.Join(home, ".edi", "agents", name+".md")
	if agent, err := loadAgentFile(globalPath); err == nil {
		return agent, nil
	}

	return nil, fmt.Errorf("agent not found: %s", name)
}

// loadAgentFile parses an agent markdown file with YAML frontmatter
func loadAgentFile(path string) (*Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	agent, body, err := parseAgentFile(content)
	if err != nil {
		return nil, err
	}

	agent.SystemPrompt = body
	return agent, nil
}

// parseAgentFile parses YAML frontmatter and markdown body
func parseAgentFile(content []byte) (*Agent, string, error) {
	reader := bufio.NewReader(bytes.NewReader(content))

	// Check for frontmatter delimiter
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(firstLine) != "---" {
		// No frontmatter, entire content is system prompt
		agent := &Agent{}
		return agent, string(content), nil
	}

	// Read frontmatter until closing ---
	var frontmatter strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, "", fmt.Errorf("unterminated frontmatter: %w", err)
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
	}

	// Parse YAML frontmatter
	var agent Agent
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &agent); err != nil {
		return nil, "", fmt.Errorf("invalid frontmatter: %w", err)
	}

	// Rest is markdown body
	var body strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		body.WriteString(line)
	}
	// Get any remaining content
	remaining := make([]byte, 1024)
	n, _ := reader.Read(remaining)
	if n > 0 {
		body.Write(remaining[:n])
	}

	return &agent, strings.TrimSpace(body.String()), nil
}

// ListAgents returns all available agent names
func ListAgents() ([]string, error) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	agents := make(map[string]bool)

	// List global agents
	globalDir := filepath.Join(home, ".edi", "agents")
	if entries, err := os.ReadDir(globalDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				name := strings.TrimSuffix(e.Name(), ".md")
				agents[name] = true
			}
		}
	}

	// List project agents (may override)
	projectDir := filepath.Join(cwd, ".edi", "agents")
	if entries, err := os.ReadDir(projectDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				name := strings.TrimSuffix(e.Name(), ".md")
				agents[name] = true
			}
		}
	}

	result := make([]string, 0, len(agents))
	for name := range agents {
		result = append(result, name)
	}
	return result, nil
}
