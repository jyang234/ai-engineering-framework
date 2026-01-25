package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/edi/internal/briefing"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Manage session history",
}

var historyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent sessions",
	RunE:  runHistoryList,
}

var historyShowCmd = &cobra.Command{
	Use:   "show [session-id]",
	Short: "Show session details",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistoryShow,
}

func init() {
	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyShowCmd)

	historyListCmd.Flags().Int("limit", 10, "Number of sessions to show")
}

func runHistoryList(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	entries, err := briefing.LoadRecentHistory(cwd, limit)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No session history found.")
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No session history found.")
		return nil
	}

	fmt.Printf("Recent Sessions (%d):\n\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s  %-12s  %s\n",
			e.Date.Format("2006-01-02 15:04"),
			e.Agent,
			e.SessionID[:8])
		if e.Summary != "" {
			lines := strings.Split(e.Summary, "\n")
			for i, line := range lines {
				if i >= 2 {
					break
				}
				fmt.Printf("    %s\n", strings.TrimSpace(line))
			}
		}
		fmt.Println()
	}

	return nil
}

func runHistoryShow(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	historyDir := filepath.Join(cwd, ".edi", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return fmt.Errorf("no history found: %w", err)
	}

	// Find matching file
	for _, entry := range entries {
		if strings.Contains(entry.Name(), sessionID) && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(historyDir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Println(string(content))
			return nil
		}
	}

	return fmt.Errorf("session not found: %s", sessionID)
}
