package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/recall"
)

var recallServerCmd = &cobra.Command{
	Use:    "recall-server",
	Short:  "Run RECALL MCP server (internal use)",
	Hidden: true,
	RunE:   runRecallServer,
}

func init() {
	recallServerCmd.Flags().String("global-db", "", "Path to global database")
	recallServerCmd.Flags().String("project-db", "", "Path to project database")
	recallServerCmd.Flags().String("session-id", "", "Session ID")
}

func runRecallServer(cmd *cobra.Command, args []string) error {
	globalDB, _ := cmd.Flags().GetString("global-db")
	projectDB, _ := cmd.Flags().GetString("project-db")
	sessionID, _ := cmd.Flags().GetString("session-id")

	if sessionID == "" {
		return fmt.Errorf("--session-id is required")
	}

	// Determine which database to use (prefer project, fallback to global)
	dbPath := projectDB
	if dbPath == "" {
		dbPath = globalDB
	}
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = home + "/.edi/recall/global.db"
	}

	// Initialize storage
	storage, err := recall.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	// Create and run server
	server := recall.NewServer(storage, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	return server.Run(ctx)
}
