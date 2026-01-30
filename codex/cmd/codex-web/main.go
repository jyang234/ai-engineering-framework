package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/web"
)

var Version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("codex-web version %s starting...", Version)

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

	// Create and run web server
	addr := getEnv("CODEX_WEB_ADDR", ":8080")
	var serverOpts []web.ServerOption
	if apiKey := os.Getenv("CODEX_API_KEY"); apiKey != "" {
		serverOpts = append(serverOpts, web.WithAPIKey(apiKey))
		log.Println("API key authentication enabled")
	}
	server := web.NewServer(engine, serverOpts...)

	log.Printf("Starting web server on %s", addr)
	if err := server.Run(addr); err != nil {
		log.Fatalf("Web server error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
