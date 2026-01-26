package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/web"
)

var (
	serveAddr string
	serveMCP  bool
	serveWeb  bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server and/or web UI",
	Long: `Start the Codex servers.

Examples:
  codex-cli serve --web --addr :8080
  codex-cli serve --mcp
  codex-cli serve --web --mcp`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "web server address")
	serveCmd.Flags().BoolVar(&serveMCP, "mcp", false, "start MCP server (stdio)")
	serveCmd.Flags().BoolVar(&serveWeb, "web", false, "start web UI server")
}

func runServe(cmd *cobra.Command, args []string) error {
	if !serveMCP && !serveWeb {
		return fmt.Errorf("specify --mcp and/or --web")
	}

	cfg := LoadConfig()
	ctx := context.Background()

	engine, err := core.NewSearchEngine(ctx, cfg.ToEngineConfig())
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer engine.Close()

	if serveMCP && serveWeb {
		return fmt.Errorf("running both MCP and web server together not yet implemented - run separately")
	}

	if serveMCP {
		fmt.Println("MCP server mode - use cmd/recall-mcp for full MCP server")
		return fmt.Errorf("MCP server available via: go run ./cmd/recall-mcp")
	}

	if serveWeb {
		fmt.Printf("Starting web server at http://localhost%s\n", serveAddr)
		server := web.NewServer(engine)
		return server.Run(serveAddr)
	}

	return nil
}
