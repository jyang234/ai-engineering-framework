package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Load loads and merges configuration from global and project sources
func Load() (*Config, error) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil // Return defaults if no home dir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cfg, nil // Return defaults if no cwd
	}

	// Load global config first
	globalPath := filepath.Join(home, ".edi", "config.yaml")
	if err := loadFile(globalPath, cfg); err != nil && !os.IsNotExist(err) {
		// Log warning but continue with defaults
	}

	// Load project config (overrides global)
	projectPath := filepath.Join(cwd, ".edi", "config.yaml")
	if err := loadFile(projectPath, cfg); err != nil && !os.IsNotExist(err) {
		// Log warning but continue
	}

	// Auto-detect project name if not set
	if cfg.Project.Name == "" {
		cfg.Project.Name = filepath.Base(cwd)
	}

	return cfg, nil
}

func loadFile(path string, cfg *Config) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	return v.Unmarshal(cfg)
}

// GlobalConfigPath returns the path to the global config file
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".edi", "config.yaml")
}

// ProjectConfigPath returns the path to the project config file
func ProjectConfigPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".edi", "config.yaml")
}

// GlobalEdiPath returns the path to the global EDI directory
func GlobalEdiPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".edi")
}

// ProjectEdiPath returns the path to the project EDI directory
func ProjectEdiPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".edi")
}
