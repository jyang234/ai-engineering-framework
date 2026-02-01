package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/config"
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
	recallAddCmd.Flags().Bool("if-not-exists", false, "Skip if an item with the same title already exists")
}

func runRecallSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	types, _ := cmd.Flags().GetStringSlice("type")
	scope, _ := cmd.Flags().GetString("scope")
	limit, _ := cmd.Flags().GetInt("limit")

	backend, err := openRecallBackend()
	if err != nil {
		return err
	}
	defer backend.Close()

	results, err := backend.Search(query, types, scope, limit)
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

	cfg, _ := config.Load()

	fmt.Println("RECALL Status:")
	fmt.Printf("  Backend: %s\n", cfg.Recall.Backend)
	fmt.Println()

	if cfg.Recall.Backend == "codex" {
		dbPath := expandTilde(cfg.Codex.MetadataDB, home)
		if dbPath == "" {
			dbPath = filepath.Join(home, ".edi", "codex.db")
		}
		fmt.Printf("Codex DB: %s\n", dbPath)
		if _, err := os.Stat(dbPath); err == nil {
			fmt.Println("  Status: OK")
		} else {
			fmt.Println("  Status: Not initialized")
		}
	} else {
		globalDB := filepath.Join(home, ".edi", "recall", "global.db")
		projectDB := filepath.Join(cwd, ".edi", "recall", "project.db")

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

	ifNotExists, _ := cmd.Flags().GetBool("if-not-exists")

	backend, err := openRecallBackend()
	if err != nil {
		return err
	}
	defer backend.Close()

	if ifNotExists {
		existing, err := backend.FindByTitle(title)
		if err == nil && existing != nil {
			fmt.Printf("Skipped (already exists): %s (id: %s)\n", title, existing.ID)
			return nil
		}
	}

	now := time.Now()
	cwd, _ := os.Getwd()

	item := &recall.Item{
		ID:          uuid.New().String(),
		Type:        itemType,
		Title:       title,
		Content:     content,
		Tags:        tags,
		Scope:       scope,
		ProjectPath: cwd,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := backend.Add(item); err != nil {
		return fmt.Errorf("failed to add item: %w", err)
	}

	fmt.Printf("Added %s: %s (id: %s)\n", itemType, title, item.ID)
	return nil
}

// expandTilde replaces a leading ~ with the home directory and resolves relative paths.
func expandTilde(path, home string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		path = filepath.Join(home, path[1:])
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	return path
}

// openRecallBackend returns the appropriate Backend based on config.
func openRecallBackend() (recall.Backend, error) {
	cfg, _ := config.Load()

	if cfg.Recall.Backend == "codex" {
		home, _ := os.UserHomeDir()
		dbPath := expandTilde(cfg.Codex.MetadataDB, home)
		if dbPath == "" {
			dbPath = filepath.Join(home, ".edi", "codex.db")
		}
		// Use FTS-only for CLI commands (no Ollama dependency)
		return recall.NewCodexBackend(dbPath, true)
	}

	// v0 backend
	cwd, _ := os.Getwd()

	// Try project DB first
	projectDB := filepath.Join(cwd, ".edi", "recall", "project.db")
	if _, err := os.Stat(filepath.Dir(projectDB)); err == nil {
		s, err := recall.NewStorage(projectDB)
		if err != nil {
			return nil, err
		}
		return s, nil
	}

	// Fall back to global DB
	home, _ := os.UserHomeDir()
	globalDB := filepath.Join(home, ".edi", "recall", "global.db")
	s, err := recall.NewStorage(globalDB)
	if err != nil {
		return nil, err
	}
	return s, nil
}
