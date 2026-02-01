package flightlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Entry represents a raw JSONL line from Claude Code transcripts.
type Entry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	UUID      string          `json:"uuid"`
	Message   json.RawMessage `json:"message"`
	Data      json.RawMessage `json:"data"`
	Summary   string          `json:"summary"`
	IsMeta    bool            `json:"isMeta"`
	Slug      string          `json:"slug"`
	Subtype   string          `json:"subtype"`
	CWD       string          `json:"cwd"`
	GitBranch string          `json:"gitBranch"`
}

// MessageContent represents the message field in user/assistant entries.
type MessageContent struct {
	Role    string        `json:"role"`
	Content json.RawMessage `json:"content"`
	Model   string        `json:"model"`
}

// ContentBlock represents a single block in the content array.
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

// Event is a parsed, display-ready event from the transcript.
type Event struct {
	Timestamp time.Time
	Category  string // "tool", "mcp", "user", "output", "mode_switch", "system", "summary"
	Summary   string // one-line description
	Detail    string // full content for --verbose
}

// ParseFile reads a JSONL file and returns structured events.
func ParseFile(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}

		evts := extractEvents(entry)
		events = append(events, evts...)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return events, nil
}

func extractEvents(entry Entry) []Event {
	ts := parseTimestamp(entry.Timestamp)

	switch entry.Type {
	case "assistant":
		return extractAssistantEvents(entry, ts)
	case "user":
		return extractUserEvents(entry, ts)
	case "system":
		return extractSystemEvents(entry, ts)
	case "summary":
		if entry.Summary != "" {
			return []Event{{
				Timestamp: ts,
				Category:  "summary",
				Summary:   truncate(entry.Summary, 120),
				Detail:    entry.Summary,
			}}
		}
	}

	return nil
}

func extractAssistantEvents(entry Entry, ts time.Time) []Event {
	var msg MessageContent
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil
	}

	var blocks []ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		// content might be a string
		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil && text != "" {
			return []Event{{
				Timestamp: ts,
				Category:  "output",
				Summary:   truncate(firstLine(text), 120),
				Detail:    text,
			}}
		}
		return nil
	}

	var events []Event
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				events = append(events, Event{
					Timestamp: ts,
					Category:  "output",
					Summary:   truncate(firstLine(b.Text), 120),
					Detail:    b.Text,
				})
			}
		case "tool_use":
			evt := makeToolEvent(b, ts)
			events = append(events, evt)
		}
	}

	return events
}

func extractUserEvents(entry Entry, ts time.Time) []Event {
	var msg MessageContent
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil
	}

	// Check if content is a string (direct user input)
	var text string
	if err := json.Unmarshal(msg.Content, &text); err == nil && text != "" {
		cat := "user"
		if entry.IsMeta {
			cat = "mode_switch"
		}
		return []Event{{
			Timestamp: ts,
			Category:  cat,
			Summary:   truncate(firstLine(text), 120),
			Detail:    text,
		}}
	}

	// Content is an array (tool results, etc.)
	var blocks []ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}

	var events []Event
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				cat := "user"
				if entry.IsMeta {
					cat = "mode_switch"
				}
				events = append(events, Event{
					Timestamp: ts,
					Category:  cat,
					Summary:   truncate(firstLine(b.Text), 120),
					Detail:    b.Text,
				})
			}
		case "tool_result":
			// Skip tool results in compact mode (they echo tool calls)
		}
	}

	return events
}

func extractSystemEvents(entry Entry, ts time.Time) []Event {
	summary := entry.Subtype
	if entry.Slug != "" {
		summary = entry.Slug + " (" + entry.Subtype + ")"
	}
	return []Event{{
		Timestamp: ts,
		Category:  "system",
		Summary:   summary,
	}}
}

func makeToolEvent(b ContentBlock, ts time.Time) Event {
	name := b.Name
	category := "tool"
	if strings.HasPrefix(name, "mcp__recall__") {
		category = "mcp"
		name = strings.TrimPrefix(name, "mcp__recall__")
	}

	summary := summarizeTool(b.Name, b.Input)

	return Event{
		Timestamp: ts,
		Category:  category,
		Summary:   summary,
		Detail:    string(b.Input),
	}
}

func summarizeTool(name string, input json.RawMessage) string {
	var m map[string]interface{}
	json.Unmarshal(input, &m)

	switch name {
	case "Read":
		if p, ok := m["file_path"].(string); ok {
			return fmt.Sprintf("Read %s", shortPath(p))
		}
	case "Edit":
		if p, ok := m["file_path"].(string); ok {
			return fmt.Sprintf("Edit %s", shortPath(p))
		}
	case "Write":
		if p, ok := m["file_path"].(string); ok {
			return fmt.Sprintf("Write %s", shortPath(p))
		}
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			return fmt.Sprintf("Bash: %s", truncate(firstLine(cmd), 100))
		}
	case "Glob":
		if p, ok := m["pattern"].(string); ok {
			return fmt.Sprintf("Glob %s", p)
		}
	case "Grep":
		if p, ok := m["pattern"].(string); ok {
			return fmt.Sprintf("Grep %q", truncate(p, 60))
		}
	case "Task":
		if d, ok := m["description"].(string); ok {
			return fmt.Sprintf("Task: %s", d)
		}
	case "mcp__recall__recall_search":
		if q, ok := m["query"].(string); ok {
			return fmt.Sprintf("recall_search %q", truncate(q, 60))
		}
	case "mcp__recall__recall_add":
		if t, ok := m["title"].(string); ok {
			return fmt.Sprintf("recall_add %q", truncate(t, 60))
		}
	case "mcp__recall__flight_recorder_log":
		if c, ok := m["content"].(string); ok {
			return fmt.Sprintf("flight_log: %s", truncate(c, 80))
		}
	case "mcp__recall__recall_get":
		if id, ok := m["id"].(string); ok {
			return fmt.Sprintf("recall_get %s", id)
		}
	case "mcp__recall__recall_feedback":
		return "recall_feedback"
	}

	return name
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func shortPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
