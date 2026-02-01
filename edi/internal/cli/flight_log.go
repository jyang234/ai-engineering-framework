package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/flightlog"
	"github.com/spf13/cobra"
)

var flightLogCmd = &cobra.Command{
	Use:   "flight-log [session-prefix]",
	Short: "Read Claude Code session transcripts as structured activity logs",
	Long: `Browse and inspect Claude Code JSONL transcripts.

Without arguments, lists recent sessions across all projects.
With a session prefix (EDI session ID or Claude session ID prefix), shows activity timeline.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFlightLog,
}

var (
	flightLogType    string
	flightLogCompact bool
	flightLogVerbose bool
	flightLogJSON    bool
)

func init() {
	flightLogCmd.Flags().StringVar(&flightLogType, "type", "", "Filter by event type (tool, mcp, user, output, mode_switch, system, summary)")
	flightLogCmd.Flags().BoolVar(&flightLogCompact, "compact", true, "One-line-per-event summary")
	flightLogCmd.Flags().BoolVar(&flightLogVerbose, "verbose", false, "Show full content including tool inputs/outputs")
	flightLogCmd.Flags().BoolVar(&flightLogJSON, "json", false, "Raw JSON output")
}

func runFlightLog(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return listSessions()
	}
	return showSession(args[0])
}

func listSessions() error {
	sessions, err := flightlog.FindSessions("")
	if err != nil {
		return fmt.Errorf("finding sessions: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -30)
	var recent []flightlog.SessionInfo
	for _, s := range sessions {
		if s.LastTimestamp.After(cutoff) {
			recent = append(recent, s)
		}
	}

	if len(recent) == 0 {
		fmt.Println("No sessions found in the last 30 days.")
		return nil
	}

	fmt.Printf("%-12s %-10s %-38s %6s  %s\n", "DATE", "EDI ID", "CLAUDE SESSION", "LINES", "PROJECT")
	fmt.Println(strings.Repeat("-", 110))

	for _, s := range recent {
		date := s.LastTimestamp.Local().Format("Jan 02 15:04")
		ediPrefix := "-"
		if s.EDISessionID != "" {
			if len(s.EDISessionID) > 8 {
				ediPrefix = s.EDISessionID[:8]
			} else {
				ediPrefix = s.EDISessionID
			}
		}
		claudeID := s.ClaudeSessionID
		if len(claudeID) > 36 {
			claudeID = claudeID[:36]
		}
		project := s.ProjectDir

		fmt.Printf("%-12s %-10s %-38s %6d  %s\n", date, ediPrefix, claudeID, s.LineCount, project)
	}

	fmt.Printf("\n%d sessions in the last 30 days\n", len(recent))
	return nil
}

func showSession(prefix string) error {
	// Try Claude session ID prefix first (exact filename match), then EDI session ID
	var path string
	sessions, err := flightlog.FindSessions("")
	if err != nil {
		return fmt.Errorf("finding sessions: %w", err)
	}

	// Match Claude session ID prefix
	for _, s := range sessions {
		if strings.HasPrefix(s.ClaudeSessionID, prefix) {
			path = s.FilePath
			break
		}
	}

	// Match EDI session ID prefix
	if path == "" {
		for _, s := range sessions {
			if s.EDISessionID != "" && strings.HasPrefix(s.EDISessionID, prefix) {
				path = s.FilePath
				break
			}
		}
	}

	if path == "" {
		return fmt.Errorf("no session found matching prefix %q", prefix)
	}

	events, err := flightlog.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parsing session: %w", err)
	}

	// Filter by type
	if flightLogType != "" {
		var filtered []flightlog.Event
		for _, e := range events {
			if e.Category == flightLogType {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	if len(events) == 0 {
		fmt.Println("No matching events found.")
		return nil
	}

	if flightLogJSON {
		return outputJSON(events)
	}

	return outputTimeline(events)
}

func outputJSON(events []flightlog.Event) error {
	type jsonEvent struct {
		Timestamp string `json:"timestamp"`
		Category  string `json:"category"`
		Summary   string `json:"summary"`
		Detail    string `json:"detail,omitempty"`
	}

	for _, e := range events {
		je := jsonEvent{
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Category:  e.Category,
			Summary:   e.Summary,
		}
		if flightLogVerbose {
			je.Detail = e.Detail
		}
		data, _ := json.Marshal(je)
		fmt.Println(string(data))
	}
	return nil
}

func outputTimeline(events []flightlog.Event) error {
	for _, e := range events {
		ts := ""
		if !e.Timestamp.IsZero() {
			ts = e.Timestamp.Local().Format("15:04:05")
		}

		cat := categoryLabel(e.Category)

		if flightLogVerbose && e.Detail != "" {
			fmt.Printf("%s  %s  %s\n", ts, cat, e.Summary)
			// Indent detail
			for _, line := range strings.Split(e.Detail, "\n") {
				fmt.Printf("          │ %s\n", line)
			}
			fmt.Println()
		} else {
			fmt.Printf("%s  %s  %s\n", ts, cat, e.Summary)
		}
	}

	fmt.Printf("\n%d events\n", len(events))
	return nil
}

func categoryLabel(cat string) string {
	switch cat {
	case "tool":
		return "[tool]  "
	case "mcp":
		return "[mcp]   "
	case "user":
		return "[user]  "
	case "output":
		return "[output]"
	case "mode_switch":
		return "[mode]  "
	case "system":
		return "[sys]   "
	case "summary":
		return "[sum]   "
	default:
		return "[" + cat + "]"
	}
}
