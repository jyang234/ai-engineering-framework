package config

// Config represents the full EDI configuration
type Config struct {
	Version string `yaml:"version" mapstructure:"version"`

	// Current agent mode
	Agent string `yaml:"agent" mapstructure:"agent"`

	// RECALL configuration
	Recall RecallConfig `yaml:"recall" mapstructure:"recall"`

	// Codex v1 backend configuration (when recall.backend = "codex")
	Codex CodexConfig `yaml:"codex" mapstructure:"codex"`

	// Briefing configuration
	Briefing BriefingConfig `yaml:"briefing" mapstructure:"briefing"`

	// Capture configuration
	Capture CaptureConfig `yaml:"capture" mapstructure:"capture"`

	// Tasks configuration
	Tasks TasksConfig `yaml:"tasks" mapstructure:"tasks"`

	// Auto memory integration (Claude Code MEMORY.md)
	Memory MemoryConfig `yaml:"memory" mapstructure:"memory"`

	// Project-specific settings (only in project config)
	Project ProjectConfig `yaml:"project" mapstructure:"project"`
}

// RecallConfig configures the RECALL knowledge system
type RecallConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Backend string `yaml:"backend" mapstructure:"backend"` // "v0" (default) or "codex"
}

// CodexConfig configures the Codex v1 backend (hybrid vector search)
type CodexConfig struct {
	ModelsPath   string `yaml:"models_path" mapstructure:"models_path"`     // Path to ONNX reranker models
	MetadataDB   string `yaml:"metadata_db" mapstructure:"metadata_db"`     // Path to SQLite metadata DB
	BinaryPath   string `yaml:"binary_path" mapstructure:"binary_path"`     // Path to recall-mcp binary (optional)
}

// BriefingConfig configures session briefing generation
type BriefingConfig struct {
	IncludeHistory bool `yaml:"include_history" mapstructure:"include_history"`
	HistoryEntries int  `yaml:"history_entries" mapstructure:"history_entries"`
	IncludeTasks   bool `yaml:"include_tasks" mapstructure:"include_tasks"`
	IncludeProfile bool `yaml:"include_profile" mapstructure:"include_profile"`
	IncludeStatus  bool `yaml:"include_status" mapstructure:"include_status"`
}

// CaptureConfig configures the capture workflow
type CaptureConfig struct {
	FrictionBudget int `yaml:"friction_budget" mapstructure:"friction_budget"`
}

// TasksConfig configures task integration
type TasksConfig struct {
	LazyLoading        bool `yaml:"lazy_loading" mapstructure:"lazy_loading"`
	CaptureOnComplete  bool `yaml:"capture_on_completion" mapstructure:"capture_on_completion"`
	PropagateDecisions bool `yaml:"propagate_decisions" mapstructure:"propagate_decisions"`
}

// MemoryConfig configures Claude Code auto memory (MEMORY.md) integration
type MemoryConfig struct {
	Enabled        bool `yaml:"enabled" mapstructure:"enabled"`                 // Enable auto memory management
	UpdateOnLaunch bool `yaml:"update_on_launch" mapstructure:"update_on_launch"` // Update MEMORY.md on session start
	UpdateOnEnd    bool `yaml:"update_on_end" mapstructure:"update_on_end"`       // Update MEMORY.md on session end
}

// ProjectConfig holds project-specific settings
type ProjectConfig struct {
	Name string `yaml:"name" mapstructure:"name"`
}
