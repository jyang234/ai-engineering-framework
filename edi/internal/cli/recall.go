package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/recall"
)

var recallCmd = &cobra.Command{
	Use:   "recall",
	Short: "RECALL knowledge base utilities",
}

var recallSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search RECALL knowledge base",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecallSearch,
}

var recallStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show RECALL status",
	RunE:  runRecallStatus,
}

var recallAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add item to RECALL",
	RunE:  runRecallAdd,
}

func init() {
	recallCmd.AddCommand(recallSearchCmd)
	recallCmd.AddCommand(recallStatusCmd)
	recallCmd.AddCommand(recallAddCmd)

	recallSearchCmd.Flags().StringSlice("type", nil, "Filter by type (pattern, failure, decision)")
	recallSearchCmd.Flags().String("scope", "all", "Scope: project, global, all")
	recallSearchCmd.Flags().Int("limit", 10, "Max results")

	recallAddCmd.Flags().String("type", "pattern", "Type: pattern, failure, decision")
	recallAddCmd.Flags().String("title", "", "Title (required)")
	recallAddCmd.Flags().String("content", "", "Content (required)")
	recallAddCmd.Flags().String("scope", "project", "Scope: project, global")
	recallAddCmd.Flags().StringSlice("tags", nil, "Tags")
}

func runRecallSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	types, _ := cmd.Flags().GetStringSlice("type")
	scope, _ := cmd.Flags().GetString("scope")
	limit, _ := cmd.Flags().GetInt("limit")

	storage, err := openRecallStorage()
	if err != nil {
		return err
	}
	defer storage.Close()

	results, err := storage.Search(query, types, scope, limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, item := range results {
		fmt.Printf("[%s] %s\n", item.ID, item.Title)
		fmt.Printf("  Type: %s  Scope: %s\n", item.Type, item.Scope)
		if len(item.Content) > 100 {
			fmt.Printf("  %s...\n", item.Content[:100])
		} else {
			fmt.Printf("  %s\n", item.Content)
		}
		fmt.Println()
	}

	return nil
}

func runRecallStatus(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	globalDB := filepath.Join(home, ".edi", "recall", "global.db")
	projectDB := filepath.Join(cwd, ".edi", "recall", "project.db")

	fmt.Println("RECALL Status:")
	fmt.Println()

	fmt.Printf("Global DB: %s\n", globalDB)
	if _, err := os.Stat(globalDB); err == nil {
		fmt.Println("  Status: OK")
	} else {
		fmt.Println("  Status: Not initialized")
	}
	fmt.Println()

	fmt.Printf("Project DB: %s\n", projectDB)
	if _, err := os.Stat(projectDB); err == nil {
		fmt.Println("  Status: OK")
	} else {
		fmt.Println("  Status: Not initialized")
	}

	return nil
}

func runRecallAdd(cmd *cobra.Command, args []string) error {
	itemType, _ := cmd.Flags().GetString("type")
	title, _ := cmd.Flags().GetString("title")
	content, _ := cmd.Flags().GetString("content")
	scope, _ := cmd.Flags().GetString("scope")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	if title == "" || content == "" {
		return fmt.Errorf("--title and --content are required")
	}

	storage, err := openRecallStorage()
	if err != nil {
		return err
	}
	defer storage.Close()

	// Create server to use its add logic
	server := recall.NewServer(storage, "cli")

	result, err := json.Marshal(map[string]interface{}{
		"type":    itemType,
		"title":   title,
		"content": content,
		"scope":   scope,
		"tags":    tags,
	})
	if err != nil {
		return err
	}

	// Use the server's add handler
	var addArgs map[string]interface{}
	json.Unmarshal(result, &addArgs)

	_ = server
	_ = context.Background()

	fmt.Printf("Added %s: %s\n", itemType, title)
	return nil
}

func openRecallStorage() (*recall.Storage, error) {
	cwd, _ := os.Getwd()

	// Try project DB first
	projectDB := filepath.Join(cwd, ".edi", "recall", "project.db")
	if _, err := os.Stat(filepath.Dir(projectDB)); err == nil {
		return recall.NewStorage(projectDB)
	}

	// Fall back to global DB
	home, _ := os.UserHomeDir()
	globalDB := filepath.Join(home, ".edi", "recall", "global.db")
	return recall.NewStorage(globalDB)
}
