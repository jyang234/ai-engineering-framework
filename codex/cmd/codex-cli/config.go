package main

import (
	"os"
	"path/filepath"

	"github.com/anthropics/aef/codex/internal/core"
)

// Config holds CLI configuration loaded from environment
type Config struct {
	QdrantAddr      string
	CollectionName  string
	VoyageAPIKey    string
	OpenAIAPIKey    string
	AnthropicAPIKey string
	ModelsPath      string
	MetadataDBPath  string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		QdrantAddr:      getEnv("QDRANT_ADDR", "localhost:6334"),
		CollectionName:  getEnv("CODEX_COLLECTION", "codex_v1"),
		VoyageAPIKey:    os.Getenv("VOYAGE_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ModelsPath:      os.Getenv("CODEX_MODELS_PATH"),
		MetadataDBPath:  getEnv("CODEX_METADATA_DB", defaultMetadataPath()),
	}
}

// ToEngineConfig converts to core.Config
func (c *Config) ToEngineConfig() core.Config {
	return core.Config{
		QdrantAddr:      c.QdrantAddr,
		CollectionName:  c.CollectionName,
		VoyageAPIKey:    c.VoyageAPIKey,
		OpenAIAPIKey:    c.OpenAIAPIKey,
		AnthropicAPIKey: c.AnthropicAPIKey,
		ModelsPath:      c.ModelsPath,
		MetadataDBPath:  c.MetadataDBPath,
	}
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
	// VoyageAPIKey and OpenAIAPIKey are needed for indexing but not for all operations
	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func defaultMetadataPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex/metadata.db"
	}
	return filepath.Join(home, ".codex", "metadata.db")
}
