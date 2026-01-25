package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "codex",
		Short:   "Codex - Knowledge retrieval system for EDI",
		Version: Version,
	}

	// Add subcommands
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(serveCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Index a file or directory into Codex",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement indexing
			fmt.Printf("Indexing: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringP("type", "t", "", "Content type (code, doc, manual)")
	cmd.Flags().StringP("scope", "s", "project", "Scope (global, project)")

	return cmd
}

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search the knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement search
			fmt.Printf("Searching: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringSliceP("types", "t", nil, "Filter by types")
	cmd.Flags().IntP("limit", "n", 10, "Maximum results")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON")

	return cmd
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from RECALL v0 (SQLite FTS) to Codex v1 (Qdrant)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement migration
			fmt.Println("Starting migration from v0 to v1...")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Codex system status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status
			fmt.Println("Codex Status")
			fmt.Println("============")
			fmt.Println("Qdrant: checking...")
			fmt.Println("Models: checking...")
			return nil
		},
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server and/or web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement serve
			fmt.Println("Starting Codex servers...")
			return nil
		},
	}

	cmd.Flags().Bool("mcp", true, "Start MCP server")
	cmd.Flags().Bool("web", false, "Start web UI")
	cmd.Flags().String("web-addr", ":8080", "Web server address")

	return cmd
}
