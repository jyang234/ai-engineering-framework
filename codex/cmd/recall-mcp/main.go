package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/mcp"
)

var Version = "dev"

func main() {
	// Structured JSON logging to stderr (stdout is reserved for MCP protocol)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("recall-mcp starting", "version", Version)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down")
		cancel()
	}()

	// Initialize search engine
	engine, err := core.NewSearchEngine(ctx, core.Config{
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		ModelsPath:          getEnv("CODEX_MODELS_PATH", "./models"),
		MetadataDBPath:      getEnv("CODEX_METADATA_DB", defaultMetadataDB()),
		LocalEmbeddingURL:   os.Getenv("LOCAL_EMBEDDING_URL"),
		LocalEmbeddingModel: os.Getenv("LOCAL_EMBEDDING_MODEL"),
	})
	if err != nil {
		log.Fatalf("Failed to initialize search engine: %v", err)
	}
	defer engine.Close()

	// Startup health check — informational, not blocking
	status := engine.HealthCheck(ctx)
	slog.Info("health check",
		"db_healthy", status.DBHealthy,
		"embedding_healthy", status.EmbeddingHealthy,
		"vectors", status.VectorCount,
		"items", status.ItemCount,
	)
	if status.EmbeddingError != "" {
		slog.Warn("embedding service unavailable — search will be keyword-only",
			"error", status.EmbeddingError)
	}

	// Get session ID from environment (passed by EDI)
	sessionID := getEnv("EDI_SESSION_ID", "unknown")

	// Create and run MCP server
	server := mcp.NewServer(engine, sessionID)
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("MCP server error: %v", err)
	}
}

func defaultMetadataDB() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "codex.db"
	}
	return filepath.Join(home, ".edi", "codex.db")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
