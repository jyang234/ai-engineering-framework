package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/mcp"
)

var Version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("recall-mcp version %s starting...", Version)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Initialize search engine
	engine, err := core.NewSearchEngine(ctx, core.Config{
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		ModelsPath:             getEnv("CODEX_MODELS_PATH", "./models"),
		MetadataDBPath:         getEnv("CODEX_METADATA_DB", "~/.edi/codex.db"),
		LocalEmbeddingURL:   os.Getenv("LOCAL_EMBEDDING_URL"),
		LocalEmbeddingModel: os.Getenv("LOCAL_EMBEDDING_MODEL"),
	})
	if err != nil {
		log.Fatalf("Failed to initialize search engine: %v", err)
	}
	defer engine.Close()

	// Get session ID from environment (passed by EDI)
	sessionID := getEnv("EDI_SESSION_ID", "unknown")

	// Create and run MCP server
	server := mcp.NewServer(engine, sessionID)
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("MCP server error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
