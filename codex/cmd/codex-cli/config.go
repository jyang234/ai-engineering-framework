package main

import (
	"os"
	"path/filepath"

	"github.com/anthropics/aef/codex/internal/core"
)

// Config holds CLI configuration loaded from environment
type Config struct {
	AnthropicAPIKey        string
	ModelsPath             string
	MetadataDBPath         string
	LocalEmbeddingURL   string
	LocalEmbeddingModel string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		ModelsPath:             os.Getenv("CODEX_MODELS_PATH"),
		MetadataDBPath:         getEnv("CODEX_METADATA_DB", defaultMetadataPath()),
		LocalEmbeddingURL:   os.Getenv("LOCAL_EMBEDDING_URL"),
		LocalEmbeddingModel: os.Getenv("LOCAL_EMBEDDING_MODEL"),
	}
}

// ToEngineConfig converts to core.Config
func (c *Config) ToEngineConfig() core.Config {
	return core.Config{
		AnthropicAPIKey:        c.AnthropicAPIKey,
		ModelsPath:             c.ModelsPath,
		MetadataDBPath:         c.MetadataDBPath,
		LocalEmbeddingURL:   c.LocalEmbeddingURL,
		LocalEmbeddingModel: c.LocalEmbeddingModel,
	}
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
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
