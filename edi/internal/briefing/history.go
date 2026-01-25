package briefing

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// HistoryEntry represents a session history entry
type HistoryEntry struct {
	SessionID         string    `yaml:"session_id"`
	Date              time.Time `yaml:"started_at"`
	EndedAt           time.Time `yaml:"ended_at"`
	Agent             string    `yaml:"agent"`
	TasksCompleted    []string  `yaml:"tasks_completed"`
	DecisionsCaptured []string  `yaml:"decisions_captured"`
	Summary           string    `yaml:"-"` // Extracted from markdown body
}

// LoadRecentHistory loads the most recent history entries
func LoadRecentHistory(projectPath string, limit int) ([]HistoryEntry, error) {
	historyDir := filepath.Join(projectPath, ".edi", "history")

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, err
	}

	var histories []HistoryEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip flight recorder files
		if strings.Contains(entry.Name(), "-flight.") {
			continue
		}

		path := filepath.Join(historyDir, entry.Name())
		h, err := parseHistoryFile(path)
		if err != nil {
			continue
		}

		histories = append(histories, h)
	}

	// Sort by date descending
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].Date.After(histories[j].Date)
	})

	// Limit
	if len(histories) > limit {
		histories = histories[:limit]
	}

	return histories, nil
}

func parseHistoryFile(path string) (HistoryEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return HistoryEntry{}, err
	}

	reader := bufio.NewReader(bytes.NewReader(content))

	// Check for frontmatter delimiter
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return HistoryEntry{}, err
	}

	if strings.TrimSpace(firstLine) != "---" {
		return HistoryEntry{}, fmt.Errorf("invalid history format: missing frontmatter")
	}

	// Read frontmatter until closing ---
	var frontmatter strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return HistoryEntry{}, fmt.Errorf("unterminated frontmatter: %w", err)
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
	}

	// Parse YAML frontmatter
	var entry HistoryEntry
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &entry); err != nil {
		return HistoryEntry{}, fmt.Errorf("invalid frontmatter: %w", err)
	}

	// Rest is markdown body - extract summary
	var body strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		body.WriteString(line)
	}

	entry.Summary = extractSummary(body.String())

	return entry, nil
}

// extractSummary extracts a brief summary from the history content
func extractSummary(content string) string {
	// Look for "## Accomplished" section
	lines := strings.Split(content, "\n")
	inAccomplished := false
	var summary strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## Accomplished") {
			inAccomplished = true
			continue
		}
		if inAccomplished {
			if strings.HasPrefix(line, "##") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && strings.HasPrefix(trimmed, "-") {
				summary.WriteString(trimmed + "\n")
			}
		}
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		// Fall back to first paragraph
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "---") {
				return trimmed
			}
		}
	}

	return result
}
