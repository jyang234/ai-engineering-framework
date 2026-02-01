package flightlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SessionInfo holds metadata about a Claude Code session.
type SessionInfo struct {
	ClaudeSessionID string
	EDISessionID    string
	ProjectDir      string // encoded project directory name
	FilePath        string
	FirstTimestamp  time.Time
	LastTimestamp   time.Time
	LineCount       int
}

// FindSessions scans ~/.claude/projects/ for JSONL session files.
// If projectDir is non-empty, only scans that specific project directory.
func FindSessions(projectDir string) ([]SessionInfo, error) {
	claudeDir, err := claudeProjectsDir()
	if err != nil {
		return nil, err
	}

	var patterns []string
	if projectDir != "" {
		encoded := encodeProjectPath(projectDir)
		patterns = append(patterns, filepath.Join(claudeDir, encoded, "*.jsonl"))
	} else {
		patterns = append(patterns, filepath.Join(claudeDir, "*", "*.jsonl"))
	}

	var sessions []SessionInfo
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			info, err := extractSessionInfo(path)
			if err != nil {
				continue
			}
			sessions = append(sessions, info)
		}
	}

	// Sort by most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastTimestamp.After(sessions[j].LastTimestamp)
	})

	return sessions, nil
}

// FindByEDISession finds the JSONL file containing the given EDI session ID prefix.
func FindByEDISession(prefix string) (string, error) {
	claudeDir, err := claudeProjectsDir()
	if err != nil {
		return "", err
	}

	matches, err := filepath.Glob(filepath.Join(claudeDir, "*", "*.jsonl"))
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	for _, path := range matches {
		if containsEDISession(path, prefix) {
			return path, nil
		}
	}

	return "", fmt.Errorf("no session found matching prefix %q", prefix)
}

func claudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// encodeProjectPath converts a filesystem path to Claude Code's directory naming.
// e.g. "/Users/john/code/myproject" → "-Users-john-code-myproject"
func encodeProjectPath(dir string) string {
	return strings.ReplaceAll(dir, string(os.PathSeparator), "-")
}

func extractSessionInfo(path string) (SessionInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return SessionInfo{}, err
	}
	defer f.Close()

	base := filepath.Base(path)
	claudeSessionID := strings.TrimSuffix(base, ".jsonl")
	projectDir := filepath.Base(filepath.Dir(path))

	info := SessionInfo{
		ClaudeSessionID: claudeSessionID,
		ProjectDir:      projectDir,
		FilePath:        path,
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry struct {
			Timestamp string `json:"timestamp"`
			Type      string `json:"type"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		ts := parseTimestamp(entry.Timestamp)
		if ts.IsZero() {
			continue
		}

		if info.FirstTimestamp.IsZero() || ts.Before(info.FirstTimestamp) {
			info.FirstTimestamp = ts
		}
		if ts.After(info.LastTimestamp) {
			info.LastTimestamp = ts
		}

		// Look for EDI session ID in early lines (UUID format only)
		if lineCount <= 100 && info.EDISessionID == "" {
			raw := string(line)
			if idx := strings.Index(raw, "Session ID: "); idx >= 0 {
				sub := raw[idx+12:]
				if m := uuidRe.FindString(sub); m != "" {
					info.EDISessionID = m
				}
			}
		}
	}

	info.LineCount = lineCount
	return info, scanner.Err()
}

var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func containsEDISession(path, prefix string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	linesRead := 0
	for scanner.Scan() {
		linesRead++
		if linesRead > 200 {
			break // EDI session ID should appear early
		}
		if strings.Contains(scanner.Text(), prefix) {
			return true
		}
	}
	return false
}
