package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/codex/internal/core"
)

var (
	searchLimit  int
	searchTypes  []string
	searchScope  string
	searchJSON   bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the knowledge base",
	Long: `Search the Codex knowledge base using hybrid search (vector + BM25).

Examples:
  codex-cli search "authentication patterns"
  codex-cli search "error handling" --type pattern --limit 5
  codex-cli search "API design" --json`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "maximum results")
	searchCmd.Flags().StringSliceVarP(&searchTypes, "type", "t", nil, "filter by type (pattern, failure, decision, code, doc)")
	searchCmd.Flags().StringVarP(&searchScope, "scope", "s", "", "filter by scope (global, project)")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output as JSON")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	cfg := LoadConfig()
	ctx := context.Background()

	engine, err := core.NewSearchEngine(ctx, cfg.ToEngineConfig())
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer engine.Close()

	results, err := engine.Search(ctx, core.SearchRequest{
		Query: query,
		Types: searchTypes,
		Scope: searchScope,
		Limit: searchLimit,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if searchJSON {
		return outputJSON(query, results)
	}
	return outputHuman(query, results)
}

func outputJSON(query string, results []core.SearchResult) error {
	output := struct {
		Query   string              `json:"query"`
		Count   int                 `json:"count"`
		Results []core.SearchResult `json:"results"`
	}{
		Query:   query,
		Count:   len(results),
		Results: results,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputHuman(query string, results []core.SearchResult) error {
	if len(results) == 0 {
		fmt.Printf("No results for \"%s\"\n", query)
		return nil
	}

	fmt.Printf("Results for \"%s\" (%d results)\n\n", query, len(results))

	for i, r := range results {
		// Type badge and title
		fmt.Printf("%d. [%s] %s", i+1, r.Type, r.Title)
		if r.Score > 0 {
			fmt.Printf("  (score: %.2f)", r.Score)
		}
		fmt.Println()

		// Metadata line
		var meta []string
		if r.Scope != "" {
			meta = append(meta, r.Scope)
		}
		if len(r.Tags) > 0 {
			meta = append(meta, "tags: "+strings.Join(r.Tags, ", "))
		}
		if len(meta) > 0 {
			fmt.Printf("   %s\n", strings.Join(meta, " | "))
		}

		// Content preview (first 200 chars)
		preview := r.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Printf("   %s\n", preview)

		// Source if available
		if r.Source != "" {
			fmt.Printf("   Source: %s\n", r.Source)
		}

		fmt.Println()
	}

	return nil
}
